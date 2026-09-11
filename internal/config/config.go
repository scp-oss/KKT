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
	// AdminPassword is the plaintext password used to log into the web UI.
	// It is hashed in memory on startup and never stored in plaintext.
	AdminPassword string
	// CookieSecure controls the Secure flag on session cookies. Disable
	// only when serving over plain HTTP (e.g. local testing).
	CookieSecure bool
}

func Load() Config {
	cfg := Config{
		ListenAddr:    getEnv("LISTEN_ADDR", ":8080"),
		DBPath:        getEnv("DB_PATH", "data/kkt.db"),
		AdminPassword: getEnv("ADMIN_PASSWORD", ""),
		CookieSecure:  getEnvBool("COOKIE_SECURE", true),
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
