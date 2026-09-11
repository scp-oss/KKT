package telegram

import (
	"testing"

	"kkt-monitor/internal/db"
)

func TestParseSocks5(t *testing.T) {
	cases := []struct {
		name       string
		in         string
		wantAddr   string
		wantUser   string
		wantPass   string
		wantNoAuth bool
		wantErr    bool
	}{
		{
			name:     "full url with auth",
			in:       "socks5://user:pass@host:1080",
			wantAddr: "host:1080",
			wantUser: "user",
			wantPass: "pass",
		},
		{
			name:       "no scheme, no auth",
			in:         "host:1080",
			wantAddr:   "host:1080",
			wantNoAuth: true,
		},
		{
			name:       "scheme, no auth",
			in:         "socks5://host:1080",
			wantAddr:   "host:1080",
			wantNoAuth: true,
		},
		{
			name:    "empty",
			in:      "",
			wantErr: true,
		},
		{
			name:    "missing host",
			in:      "socks5://",
			wantErr: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			addr, auth, err := parseSocks5(c.in)
			if c.wantErr {
				if err == nil {
					t.Fatalf("parseSocks5(%q): expected error, got addr=%q auth=%v", c.in, addr, auth)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseSocks5(%q): unexpected error: %v", c.in, err)
			}
			if addr != c.wantAddr {
				t.Errorf("addr = %q, want %q", addr, c.wantAddr)
			}
			if c.wantNoAuth {
				if auth != nil {
					t.Errorf("auth = %+v, want nil", auth)
				}
				return
			}
			if auth == nil {
				t.Fatalf("auth = nil, want User=%q Password=%q", c.wantUser, c.wantPass)
			}
			if auth.User != c.wantUser || auth.Password != c.wantPass {
				t.Errorf("auth = %+v, want User=%q Password=%q", auth, c.wantUser, c.wantPass)
			}
		})
	}
}

func TestNormalizeBaseURL(t *testing.T) {
	cases := []struct{ in, want string }{
		{"example.tld", "https://example.tld"},
		{"https://example.tld", "https://example.tld"},
		{"https://example.tld/", "https://example.tld"},
		{"  example.tld  ", "https://example.tld"},
		{"http://example.tld", "http://example.tld"},
		{"", ""},
	}
	for _, c := range cases {
		if got := normalizeBaseURL(c.in); got != c.want {
			t.Errorf("normalizeBaseURL(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestEndpoint(t *testing.T) {
	cases := []struct {
		name     string
		settings db.BotSettings
		want     string
		wantErr  bool
	}{
		{
			name:     "direct",
			settings: db.BotSettings{Mode: db.ModeDirect, Token: "123:ABC"},
			want:     "https://api.telegram.org/bot123:ABC/sendMessage",
		},
		{
			name:     "socks5 still talks to api.telegram.org",
			settings: db.BotSettings{Mode: db.ModeSocks5, Token: "123:ABC"},
			want:     "https://api.telegram.org/bot123:ABC/sendMessage",
		},
		{
			name:     "relay with auth key, bare domain",
			settings: db.BotSettings{Mode: db.ModeRelay, Token: "123:ABC", RelayBaseURL: "example.tld", AuthKey: "secret"},
			want:     "https://example.tld/bot123:ABC/sendMessage?auth=secret",
		},
		{
			name:     "relay without auth key",
			settings: db.BotSettings{Mode: db.ModeRelay, Token: "123:ABC", RelayBaseURL: "https://example.tld/"},
			want:     "https://example.tld/bot123:ABC/sendMessage",
		},
		{
			name:     "relay missing base url",
			settings: db.BotSettings{Mode: db.ModeRelay, Token: "123:ABC"},
			wantErr:  true,
		},
		{
			name:     "missing token",
			settings: db.BotSettings{Mode: db.ModeDirect},
			wantErr:  true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := New(c.settings)
			got, err := s.endpoint()
			if c.wantErr {
				if err == nil {
					t.Fatalf("endpoint(): expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("endpoint(): unexpected error: %v", err)
			}
			if got != c.want {
				t.Errorf("endpoint() = %q, want %q", got, c.want)
			}
		})
	}
}
