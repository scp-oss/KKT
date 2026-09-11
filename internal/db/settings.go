package db

// BotSettings holds the Telegram delivery configuration. Secrets (Token, AuthKey,
// CustomURLTemplate, Socks5*) are stored server-side only and are never re-rendered
// into HTML once set — the settings form always shows them blank.
type BotSettings struct {
	Mode              string // "direct" or "custom"
	Token             string
	AuthKey           string // used by custom relay endpoints, e.g. ?auth=...
	CustomURLTemplate string // must contain {TOKEN}; may contain {AUTH_KEY}
	Socks5Enabled     bool
	Socks5Addr        string
	Socks5User        string
	Socks5Pass        string
}

var settingsKeys = []string{
	"bot_mode", "bot_token", "bot_auth_key", "bot_custom_url_template",
	"socks5_enabled", "socks5_addr", "socks5_user", "socks5_pass",
}

func (d *DB) GetSetting(key string) (string, error) {
	var v string
	err := d.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&v)
	if err != nil {
		return "", nil // treat missing as empty
	}
	return v, nil
}

func (d *DB) SetSetting(key, value string) error {
	_, err := d.Exec(`INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

func (d *DB) GetBotSettings() (BotSettings, error) {
	vals := map[string]string{}
	for _, k := range settingsKeys {
		v, err := d.GetSetting(k)
		if err != nil {
			return BotSettings{}, err
		}
		vals[k] = v
	}
	s := BotSettings{
		Mode:              vals["bot_mode"],
		Token:             vals["bot_token"],
		AuthKey:           vals["bot_auth_key"],
		CustomURLTemplate: vals["bot_custom_url_template"],
		Socks5Enabled:     vals["socks5_enabled"] == "1",
		Socks5Addr:        vals["socks5_addr"],
		Socks5User:        vals["socks5_user"],
		Socks5Pass:        vals["socks5_pass"],
	}
	if s.Mode == "" {
		s.Mode = "direct"
	}
	return s, nil
}

// SaveBotSettings persists non-empty fields. Blank secret fields are left
// untouched so the settings form can be submitted without re-typing secrets
// that shouldn't change.
func (d *DB) SaveBotSettings(mode string, token, authKey, customURL *string, socks5Enabled bool, socks5Addr string, socks5User, socks5Pass *string) error {
	if err := d.SetSetting("bot_mode", mode); err != nil {
		return err
	}
	if err := d.setSettingBool("socks5_enabled", socks5Enabled); err != nil {
		return err
	}
	if err := d.SetSetting("socks5_addr", socks5Addr); err != nil {
		return err
	}
	for key, val := range map[string]*string{
		"bot_token":               token,
		"bot_auth_key":            authKey,
		"bot_custom_url_template": customURL,
		"socks5_user":             socks5User,
		"socks5_pass":             socks5Pass,
	} {
		if val != nil && *val != "" {
			if err := d.SetSetting(key, *val); err != nil {
				return err
			}
		}
	}
	return nil
}

func (d *DB) setSettingBool(key string, v bool) error {
	val := "0"
	if v {
		val = "1"
	}
	return d.SetSetting(key, val)
}
