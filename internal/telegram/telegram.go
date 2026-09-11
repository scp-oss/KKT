// Package telegram sends notification messages through the Telegram Bot API:
// directly, through a SOCKS5 proxy, or through a custom relay/mirror endpoint
// (useful where api.telegram.org itself is blocked).
package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/proxy"

	"kkt-monitor/internal/db"
)

const directBaseURL = "https://api.telegram.org"

// Sender sends messages using the given bot settings.
type Sender struct {
	settings db.BotSettings
}

func New(settings db.BotSettings) *Sender {
	return &Sender{settings: settings}
}

func (s *Sender) client() (*http.Client, error) {
	transport := &http.Transport{}

	if s.settings.Mode == db.ModeSocks5 {
		addr, auth, err := parseSocks5(s.settings.Socks5URL)
		if err != nil {
			return nil, err
		}
		dialer, err := proxy.SOCKS5("tcp", addr, auth, proxy.Direct)
		if err != nil {
			return nil, fmt.Errorf("настройка SOCKS5: %w", err)
		}
		contextDialer, ok := dialer.(proxy.ContextDialer)
		if !ok {
			return nil, fmt.Errorf("SOCKS5-прокси не поддерживает контекст")
		}
		transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return contextDialer.DialContext(ctx, network, addr)
		}
	}

	return &http.Client{Transport: transport, Timeout: 15 * time.Second}, nil
}

// parseSocks5 accepts "socks5://user:pass@host:port" (scheme optional) and
// returns the dial address plus optional auth.
func parseSocks5(raw string) (addr string, auth *proxy.Auth, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil, fmt.Errorf("не задан адрес SOCKS5-прокси")
	}
	if !strings.Contains(raw, "://") {
		raw = "socks5://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", nil, fmt.Errorf("некорректный адрес SOCKS5-прокси, ожидается host:port или socks5://user:pass@host:port")
	}
	if u.User != nil {
		password, _ := u.User.Password()
		auth = &proxy.Auth{User: u.User.Username(), Password: password}
	}
	return u.Host, auth, nil
}

// endpoint builds the sendMessage URL. Every relay that mirrors the Bot API
// exposes the same /bot{TOKEN}/sendMessage path Telegram itself uses, so the
// admin only supplies the base domain - not the whole URL - for relay mode.
func (s *Sender) endpoint() (string, error) {
	if s.settings.Token == "" {
		return "", fmt.Errorf("не задан токен бота")
	}

	base := directBaseURL
	if s.settings.Mode == db.ModeRelay {
		base = normalizeBaseURL(s.settings.RelayBaseURL)
		if base == "" {
			return "", fmt.Errorf("не задан адрес relay-сервера")
		}
	}

	endpoint := base + "/bot" + url.PathEscape(s.settings.Token) + "/sendMessage"
	if s.settings.Mode == db.ModeRelay && s.settings.AuthKey != "" {
		endpoint += "?auth=" + url.QueryEscape(s.settings.AuthKey)
	}
	return endpoint, nil
}

// normalizeBaseURL accepts a bare domain ("example.tld") or a full base URL
// ("https://example.tld/") and returns "https://example.tld" - a scheme, no
// trailing slash.
func normalizeBaseURL(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimRight(raw, "/")
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	return raw
}

// Send delivers text to a single chat id.
func (s *Sender) Send(ctx context.Context, chatID, text string) error {
	endpoint, err := s.endpoint()
	if err != nil {
		return err
	}
	client, err := s.client()
	if err != nil {
		return err
	}

	body, err := json.Marshal(map[string]string{"chat_id": chatID, "text": text})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("запрос к Telegram: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("Telegram вернул статус %d", resp.StatusCode)
	}
	return nil
}

// SendToAll sends text to every recipient and returns the first error, if any,
// after attempting delivery to all of them.
func (s *Sender) SendToAll(ctx context.Context, recipients []db.Recipient, text string) []error {
	var errs []error
	for _, r := range recipients {
		if err := s.Send(ctx, r.ChatID, text); err != nil {
			errs = append(errs, fmt.Errorf("получатель %s: %w", r.ChatID, err))
		}
	}
	return errs
}
