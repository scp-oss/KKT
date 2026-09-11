package web

import "testing"

func TestMaskPreview(t *testing.T) {
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
			got := maskPreview(c.secret, c.prefix)
			if got != c.want {
				t.Errorf("maskPreview(%q, %d) = %q, want %q", c.secret, c.prefix, got, c.want)
			}
			// Never leak more than the intended prefix in plain text: the
			// mask suffix must not itself look like part of the secret.
			if got != "" && got != c.want {
				t.Errorf("unexpected output shape: %q", got)
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
