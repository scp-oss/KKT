package telegram

import "testing"

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
