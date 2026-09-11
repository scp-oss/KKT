// Package telegram sends notification messages through the Telegram Bot API,
// either directly or through a custom relay/mirror endpoint (useful where
// api.telegram.org is blocked), optionally tunnelled through a SOCKS5 proxy.
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

const defaultDirectURLTemplate = "https://api.telegram.org/bot{TOKEN}/sendMessage"

// Sender sends messages using the given bot settings.
type Sender struct {
	settings db.BotSettings
}

func New(settings db.BotSettings) *Sender {
	return &Sender{settings: settings}
}

func (s *Sender) client() (*http.Client, error) {
	transport := &http.Transport{}

	if s.settings.Socks5Enabled && s.settings.Socks5Addr != "" {
		var auth *proxy.Auth
		if s.settings.Socks5User != "" {
			auth = &proxy.Auth{User: s.settings.Socks5User, Password: s.settings.Socks5Pass}
		}
		dialer, err := proxy.SOCKS5("tcp", s.settings.Socks5Addr, auth, proxy.Direct)
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

func (s *Sender) endpoint() (string, error) {
	if s.settings.Token == "" {
		return "", fmt.Errorf("не задан токен бота")
	}

	tmpl := s.settings.CustomURLTemplate
	if s.settings.Mode != "custom" || tmpl == "" {
		tmpl = defaultDirectURLTemplate
	}

	endpoint := strings.ReplaceAll(tmpl, "{TOKEN}", url.PathEscape(s.settings.Token))
	endpoint = strings.ReplaceAll(endpoint, "{AUTH_KEY}", url.QueryEscape(s.settings.AuthKey))
	return endpoint, nil
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
