package db

import "time"

// Recipient is one Telegram delivery target. Organization, when set, scopes
// notifications to KKT records belonging to that organization only; empty
// means "all organizations".
type Recipient struct {
	ID           int64
	ChatID       string
	Name         string
	Organization string
	Enabled      bool
}

func (d *DB) AddRecipient(chatID, name, organization string) error {
	_, err := d.Exec(`INSERT INTO recipients (chat_id, name, organization, enabled, created_at) VALUES (?, ?, ?, 1, ?)`,
		chatID, name, organization, time.Now().UTC().Format(time.RFC3339))
	return err
}

func (d *DB) DeleteRecipient(id int64) error {
	_, err := d.Exec(`DELETE FROM recipients WHERE id = ?`, id)
	return err
}

func (d *DB) SetRecipientEnabled(id int64, enabled bool) error {
	_, err := d.Exec(`UPDATE recipients SET enabled = ? WHERE id = ?`, boolToInt(enabled), id)
	return err
}

func (d *DB) ListRecipients() ([]Recipient, error) {
	rows, err := d.Query(`SELECT id, chat_id, name, organization, enabled FROM recipients ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Recipient
	for rows.Next() {
		var r Recipient
		var enabled int
		if err := rows.Scan(&r.ID, &r.ChatID, &r.Name, &r.Organization, &enabled); err != nil {
			return nil, err
		}
		r.Enabled = enabled == 1
		out = append(out, r)
	}
	return out, rows.Err()
}

func (d *DB) ListEnabledRecipients() ([]Recipient, error) {
	all, err := d.ListRecipients()
	if err != nil {
		return nil, err
	}
	var out []Recipient
	for _, r := range all {
		if r.Enabled {
			out = append(out, r)
		}
	}
	return out, nil
}

// ListEnabledRecipientsForOrganization returns enabled recipients that
// should be notified about a KKT belonging to organization: those scoped to
// it specifically, plus every unscoped ("all organizations") recipient.
func (d *DB) ListEnabledRecipientsForOrganization(organization string) ([]Recipient, error) {
	all, err := d.ListEnabledRecipients()
	if err != nil {
		return nil, err
	}
	var out []Recipient
	for _, r := range all {
		if r.Organization == "" || r.Organization == organization {
			out = append(out, r)
		}
	}
	return out, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
