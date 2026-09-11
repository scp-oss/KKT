package db

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

const sessionTTL = 30 * 24 * time.Hour

// Roles a session can carry. RoleAdmin has full access; RoleViewer is
// read-only (dashboard only, no upload/delete/settings).
const (
	RoleAdmin  = "admin"
	RoleViewer = "viewer"
)

// CreateSession creates a new random session token for the given role and
// stores its expiry.
func (d *DB) CreateSession(role string) (token string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token = hex.EncodeToString(b)
	expires := time.Now().Add(sessionTTL).UTC().Format(time.RFC3339)
	_, err = d.Exec(`INSERT INTO sessions (token, role, expires_at) VALUES (?, ?, ?)`, token, role, expires)
	return token, err
}

// SessionRole returns the role stored for token and whether it is present
// and not expired.
func (d *DB) SessionRole(token string) (role string, ok bool) {
	if token == "" {
		return "", false
	}
	var expiresAt string
	err := d.QueryRow(`SELECT role, expires_at FROM sessions WHERE token = ?`, token).Scan(&role, &expiresAt)
	if err != nil {
		return "", false
	}
	t, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		return "", false
	}
	if !time.Now().Before(t) {
		return "", false
	}
	return role, true
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
