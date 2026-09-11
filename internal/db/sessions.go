package db

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

const sessionTTL = 30 * 24 * time.Hour

// CreateSession creates a new random session token and stores its expiry.
func (d *DB) CreateSession() (token string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token = hex.EncodeToString(b)
	expires := time.Now().Add(sessionTTL).UTC().Format(time.RFC3339)
	_, err = d.Exec(`INSERT INTO sessions (token, expires_at) VALUES (?, ?)`, token, expires)
	return token, err
}

// ValidSession reports whether token exists and has not expired.
func (d *DB) ValidSession(token string) bool {
	if token == "" {
		return false
	}
	var expiresAt string
	err := d.QueryRow(`SELECT expires_at FROM sessions WHERE token = ?`, token).Scan(&expiresAt)
	if err != nil {
		return false
	}
	t, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		return false
	}
	return time.Now().Before(t)
}

func (d *DB) DeleteSession(token string) error {
	_, err := d.Exec(`DELETE FROM sessions WHERE token = ?`, token)
	return err
}

// PruneExpiredSessions removes stale rows; safe to call periodically.
func (d *DB) PruneExpiredSessions() error {
	_, err := d.Exec(`DELETE FROM sessions WHERE expires_at < ?`, time.Now().UTC().Format(time.RFC3339))
	return err
}
