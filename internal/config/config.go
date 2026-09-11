// Package config loads runtime configuration from environment variables.
package config

import (
	"os"
	"strconv"
)

type Config struct {
	// ListenAddr is the address the HTTP server binds to, e.g. ":8080".
	ListenAddr string
	// DBPath is the path to the SQLite database file.
	DBPath string
	// AdminPassword grants full access: dashboard, CSV upload, deleting
	// records, and bot/recipient settings.
	AdminPassword string
	// ViewerPassword, if set, grants read-only access to the dashboard only
	// (no upload, no delete, no settings). Leave unset to disable the
	// viewer login entirely.
	ViewerPassword string
	// CookieSecure controls the Secure flag on session cookies. Browsers
	// silently drop a Secure cookie on a plain HTTP connection, which would
	// make login look like it "does nothing" (the session cookie never
	// sticks, so every request bounces back to /login) - so this defaults
	// to false and must be explicitly enabled once the service is actually
	// served over HTTPS (directly or behind a TLS-terminating proxy).
	CookieSecure bool
}

func Load() Config {
	// LISTEN_ADDR (a full "host:port" string) takes precedence when set,
	// e.g. to bind a specific interface. Otherwise the simpler PORT variable
	// picks the port on all interfaces - handy for `docker run -e PORT=...`.
	listenAddr := getEnv("LISTEN_ADDR", "")
	if listenAddr == "" {
		listenAddr = ":" + getEnv("PORT", "8080")
	}

	cfg := Config{
		ListenAddr:     listenAddr,
		DBPath:         getEnv("DB_PATH", "data/kkt.db"),
		AdminPassword:  getEnv("ADMIN_PASSWORD", ""),
		ViewerPassword: getEnv("VIEWER_PASSWORD", ""),
		CookieSecure:   getEnvBool("COOKIE_SECURE", false),
	}
	return cfg
}

func getEnv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func getEnvBool(key string, def bool) bool {
	if v, ok := os.LookupEnv(key); ok {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}
