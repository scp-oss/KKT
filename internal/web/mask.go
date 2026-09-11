package web

import "strings"

const maskDots = "••••••••"

// maskTokenPreview shows a short, recognizable prefix of a Telegram-style
// "<bot id>:<secret>" token (the id isn't sensitive on its own) followed by
// a fixed run of mask characters, so an admin can tell which bot is
// configured without the page ever holding the full secret. The fixed
// length mask never reveals the real length.
func maskTokenPreview(secret string, plainPrefix int) string {
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

// maskURLPreview shows the first few characters of a URL's host - skipping
// past a leading "http://"/"https://" scheme first, since that's not
// specific to the actual site and would otherwise eat the whole preview
// (e.g. "https://" is 8 characters on its own) - followed by a fixed run of
// mask characters.
func maskURLPreview(url string, plainPrefix int) string {
	if url == "" {
		return ""
	}
	host := strings.TrimPrefix(strings.TrimPrefix(url, "https://"), "http://")
	runes := []rune(host)
	n := plainPrefix
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
