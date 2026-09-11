package web

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"kkt-monitor/internal/notify"
	"kkt-monitor/internal/telegram"
)

func (s *Server) handleSettingsForm(w http.ResponseWriter, r *http.Request) {
	s.renderSettings(w, "", "")
}

func (s *Server) renderSettings(w http.ResponseWriter, successMsg, errorMsg string) {
	settings, err := s.store.GetBotSettings()
	if err != nil {
		http.Error(w, "ошибка чтения настроек", http.StatusInternalServerError)
		return
	}
	recipients, err := s.store.ListRecipients()
	if err != nil {
		http.Error(w, "ошибка чтения получателей", http.StatusInternalServerError)
		return
	}

	s.render(w, "settings.html", map[string]any{
		"Mode":          settings.Mode,
		"TokenSet":      settings.Token != "",
		"AuthKeySet":    settings.AuthKey != "",
		"CustomURLSet":  settings.CustomURLTemplate != "",
		"Socks5Enabled": settings.Socks5Enabled,
		"Socks5Addr":    settings.Socks5Addr,
		"Socks5UserSet": settings.Socks5User != "",
		"Socks5PassSet": settings.Socks5Pass != "",
		"Recipients":    recipients,
		"Thresholds":    notify.Thresholds,
		"Success":       successMsg,
		"Error":         errorMsg,
	})
}

func (s *Server) handleSettingsBotSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	mode := r.FormValue("mode")
	if mode != "direct" && mode != "custom" {
		mode = "direct"
	}
	socks5Enabled := r.FormValue("socks5_enabled") == "on"
	socks5Addr := r.FormValue("socks5_addr")

	token := optionalField(r.FormValue("token"))
	authKey := optionalField(r.FormValue("auth_key"))
	customURL := optionalField(r.FormValue("custom_url"))
	socks5User := optionalField(r.FormValue("socks5_user"))
	socks5Pass := optionalField(r.FormValue("socks5_pass"))

	if err := s.store.SaveBotSettings(mode, token, authKey, customURL, socks5Enabled, socks5Addr, socks5User, socks5Pass); err != nil {
		s.renderSettings(w, "", "Ошибка сохранения настроек: "+err.Error())
		return
	}
	s.renderSettings(w, "Настройки бота сохранены.", "")
}

func optionalField(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

func (s *Server) handleRecipientAdd(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	chatID := r.FormValue("chat_id")
	name := r.FormValue("name")
	if chatID == "" {
		s.renderSettings(w, "", "Укажите chat_id получателя")
		return
	}
	if err := s.store.AddRecipient(chatID, name); err != nil {
		s.renderSettings(w, "", "Ошибка добавления получателя: "+err.Error())
		return
	}
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

func (s *Server) handleRecipientDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	if err := s.store.DeleteRecipient(id); err != nil {
		http.Error(w, "ошибка удаления", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

func (s *Server) handleRecipientToggle(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	recipients, err := s.store.ListRecipients()
	if err != nil {
		http.Error(w, "ошибка чтения получателей", http.StatusInternalServerError)
		return
	}
	for _, rec := range recipients {
		if rec.ID == id {
			s.store.SetRecipientEnabled(id, !rec.Enabled)
			break
		}
	}
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

func (s *Server) handleSettingsTest(w http.ResponseWriter, r *http.Request) {
	settings, err := s.store.GetBotSettings()
	if err != nil {
		s.renderSettings(w, "", "Ошибка чтения настроек: "+err.Error())
		return
	}
	recipients, err := s.store.ListEnabledRecipients()
	if err != nil {
		s.renderSettings(w, "", "Ошибка чтения получателей: "+err.Error())
		return
	}
	if len(recipients) == 0 {
		s.renderSettings(w, "", "Нет активных получателей для тестовой отправки")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	sender := telegram.New(settings)
	errs := sender.SendToAll(ctx, recipients, "✅ Тестовое сообщение от сервиса мониторинга ККТ")
	if len(errs) > 0 {
		msg := "Ошибки при тестовой отправке: "
		for i, e := range errs {
			if i > 0 {
				msg += "; "
			}
			msg += e.Error()
		}
		s.renderSettings(w, "", msg)
		return
	}
	s.renderSettings(w, "Тестовое сообщение отправлено всем активным получателям.", "")
}
