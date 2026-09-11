package web

import "strings"

const maskDots = "••••••••"

// maskPreview shows a short, recognizable prefix of a secret (e.g. a bot
// token's numeric id) followed by a fixed run of mask characters, so an
// admin can tell which value is configured without the page ever holding
// the full secret. For a Telegram-style "<id>:<secret>" token the prefix
// extends a couple of characters past the colon; otherwise it's a flat
// character count. The fixed-length mask never reveals the real length.
func maskPreview(secret string, plainPrefix int) string {
	if secret == "" {
		return ""
	}
	n := plainPrefix
	if i := strings.IndexByte(secret, ':'); i > 0 && i+3 < len(secret) {
		n = i + 3
	}
	runes := []rune(secret)
	if n > len(runes) {
		n = len(runes)
	}
	return string(runes[:n]) + maskDots
}

// maskFull hides a secret entirely behind a fixed run of mask characters -
// used for values with no safe-to-show prefix (e.g. an auth key, or a
// SOCKS5 URL that embeds a username/password).
func maskFull(secret string) string {
	if secret == "" {
		return ""
	}
	return maskDots
}
