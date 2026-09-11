package web

import "testing"

func TestMaskTokenPreview(t *testing.T) {
	cases := []struct {
		name   string
		secret string
		prefix int
		want   string
	}{
		{"empty", "", 8, ""},
		{
			"telegram-style token shows id plus a couple secret chars",
			"123456789:AAHexampleExampleExampleExampleE",
			8,
			"123456789:AA" + maskDots,
		},
		{"no colon falls back to flat prefix", "abcdefghijklmnop", 6, "abcdef" + maskDots},
		{"shorter than prefix shows all of it", "ab", 8, "ab" + maskDots},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := maskTokenPreview(c.secret, c.prefix)
			if got != c.want {
				t.Errorf("maskTokenPreview(%q, %d) = %q, want %q", c.secret, c.prefix, got, c.want)
			}
		})
	}
}

func TestMaskURLPreview(t *testing.T) {
	cases := []struct {
		name   string
		url    string
		prefix int
		want   string
	}{
		{"empty", "", 8, ""},
		{
			"https scheme is skipped, not counted toward the prefix",
			"https://red-domage.cc.cd",
			8,
			"red-doma" + maskDots,
		},
		{
			"http scheme is skipped too",
			"http://example.tld",
			6,
			"exampl" + maskDots,
		},
		{"no scheme at all", "example.tld", 7, "example" + maskDots},
		{"host shorter than prefix shows all of it", "ex.tld", 8, "ex.tld" + maskDots},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := maskURLPreview(c.url, c.prefix)
			if got != c.want {
				t.Errorf("maskURLPreview(%q, %d) = %q, want %q", c.url, c.prefix, got, c.want)
			}
		})
	}
}

func TestMaskFull(t *testing.T) {
	if got := maskFull(""); got != "" {
		t.Errorf("maskFull(\"\") = %q, want empty", got)
	}
	if got := maskFull("supersecret"); got != maskDots {
		t.Errorf("maskFull(secret) = %q, want %q", got, maskDots)
	}
	if got := maskFull("x"); got == "x" {
		t.Errorf("maskFull leaked the real value")
	}
}
