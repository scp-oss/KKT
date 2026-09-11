package db

// Bot delivery modes: exactly one is active at a time.
const (
	ModeDirect = "direct" // straight to api.telegram.org
	ModeSocks5 = "socks5" // api.telegram.org through a SOCKS5 proxy
	ModeRelay  = "relay"  // a custom relay/mirror endpoint, e.g. a domain that mirrors the Bot API
)

// BotSettings holds the Telegram delivery configuration. Secrets (Token, AuthKey,
// RelayBaseURL, Socks5URL) are stored server-side only and are never re-rendered
// into HTML once set — the settings form always shows them blank.
type BotSettings struct {
	Mode         string // ModeDirect, ModeSocks5 or ModeRelay
	Token        string
	AuthKey      string // relay's ?auth= query parameter, if it needs one
	RelayBaseURL string // just the domain, e.g. "https://example.tld" - the /bot{TOKEN}/sendMessage?auth={AUTH_KEY} path is appended automatically
	Socks5URL    string // e.g. socks5://user:pass@host:1080
}

var settingsKeys = []string{
	"bot_mode", "bot_token", "bot_auth_key", "bot_relay_base_url", "bot_socks5_url",
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
		Mode:         vals["bot_mode"],
		Token:        vals["bot_token"],
		AuthKey:      vals["bot_auth_key"],
		RelayBaseURL: vals["bot_relay_base_url"],
		Socks5URL:    vals["bot_socks5_url"],
	}
	switch s.Mode {
	case "":
		s.Mode = ModeDirect
	case "custom":
		s.Mode = ModeRelay // legacy value from before the 3-way mode split
	}
	return s, nil
}

// SaveBotSettings persists non-empty fields. Blank secret fields are left
// untouched so the settings form can be submitted without re-typing secrets
// that shouldn't change.
func (d *DB) SaveBotSettings(mode string, token, authKey, relayBaseURL, socks5URL *string) error {
	if err := d.SetSetting("bot_mode", mode); err != nil {
		return err
	}
	for key, val := range map[string]*string{
		"bot_token":          token,
		"bot_auth_key":       authKey,
		"bot_relay_base_url": relayBaseURL,
		"bot_socks5_url":     socks5URL,
	} {
		if val != nil && *val != "" {
			if err := d.SetSetting(key, *val); err != nil {
				return err
			}
		}
	}
	return nil
}

// GetPollSchedule returns the configured daily check times (HH:MM, server
// local time). Time1 defaults to "09:00" when unset; Time2 is optional and
// empty when the registry should only be checked once a day.
func (d *DB) GetPollSchedule() (time1, time2 string, err error) {
	time1, err = d.GetSetting("poll_time_1")
	if err != nil {
		return "", "", err
	}
	if time1 == "" {
		time1 = "09:00"
	}
	time2, err = d.GetSetting("poll_time_2")
	if err != nil {
		return "", "", err
	}
	return time1, time2, nil
}

func (d *DB) SetPollSchedule(time1, time2 string) error {
	if err := d.SetSetting("poll_time_1", time1); err != nil {
		return err
	}
	return d.SetSetting("poll_time_2", time2)
}
