package web

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"kkt-monitor/internal/csvimport"
	"kkt-monitor/internal/db"
	"kkt-monitor/internal/notify"
	"kkt-monitor/internal/telegram"
)

func (s *Server) handleSettingsForm(w http.ResponseWriter, r *http.Request) {
	s.renderSettings(w, nil)
}

// renderSettings renders the settings page, merging extra (typically a
// Success/Error message, or upload-result fields) over the base data.
func (s *Server) renderSettings(w http.ResponseWriter, extra map[string]any) {
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
	organizations, err := s.store.ListOrganizations()
	if err != nil {
		http.Error(w, "ошибка чтения списка организаций", http.StatusInternalServerError)
		return
	}
	pollTime1, pollTime2, err := s.store.GetPollSchedule()
	if err != nil {
		http.Error(w, "ошибка чтения расписания уведомлений", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Mode":            settings.Mode,
		"TokenSet":        settings.Token != "",
		"AuthKeySet":      settings.AuthKey != "",
		"RelayBaseURLSet": settings.RelayBaseURL != "",
		"Socks5URLSet":    settings.Socks5URL != "",
		"PollTime1":       pollTime1,
		"PollTime2":       pollTime2,
		"Recipients":      recipients,
		"Organizations":   organizations,
		"Thresholds":      notify.Thresholds,
	}
	for k, v := range extra {
		data[k] = v
	}
	s.render(w, "settings.html", data)
}

func (s *Server) handleSettingsBotSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	mode := r.FormValue("mode")
	switch mode {
	case db.ModeDirect, db.ModeSocks5, db.ModeRelay:
	default:
		mode = db.ModeDirect
	}

	sendTime1 := strings.TrimSpace(r.FormValue("poll_time_1"))
	if sendTime1 == "" {
		sendTime1 = "09:00"
	}
	sendTime2 := strings.TrimSpace(r.FormValue("poll_time_2"))
	if _, err := time.Parse("15:04", sendTime1); err != nil {
		s.renderSettings(w, map[string]any{"Error": "Время отправки 1: неверный формат, ожидается ЧЧ:ММ"})
		return
	}
	if sendTime2 != "" {
		if _, err := time.Parse("15:04", sendTime2); err != nil {
			s.renderSettings(w, map[string]any{"Error": "Время отправки 2: неверный формат, ожидается ЧЧ:ММ"})
			return
		}
	}

	token := optionalField(r.FormValue("token"))
	authKey := optionalField(r.FormValue("auth_key"))
	relayBaseURL := optionalField(r.FormValue("relay_base_url"))
	socks5URL := optionalField(r.FormValue("socks5_url"))

	if err := s.store.SetPollSchedule(sendTime1, sendTime2); err != nil {
		s.renderSettings(w, map[string]any{"Error": "Ошибка сохранения расписания: " + err.Error()})
		return
	}
	if err := s.store.SaveBotSettings(mode, token, authKey, relayBaseURL, socks5URL); err != nil {
		s.renderSettings(w, map[string]any{"Error": "Ошибка сохранения настроек: " + err.Error()})
		return
	}
	s.renderSettings(w, map[string]any{"Success": "Настройки сохранены."})
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
	chatID := strings.TrimSpace(r.FormValue("chat_id"))
	name := strings.TrimSpace(r.FormValue("name"))
	organization := r.FormValue("organization") // "" means all organizations
	if chatID == "" {
		s.renderSettings(w, map[string]any{"Error": "Укажите chat_id получателя"})
		return
	}
	if err := s.store.AddRecipient(chatID, name, organization); err != nil {
		s.renderSettings(w, map[string]any{"Error": "Ошибка добавления получателя: " + err.Error()})
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
		s.renderSettings(w, map[string]any{"Error": "Ошибка чтения настроек: " + err.Error()})
		return
	}
	recipients, err := s.store.ListEnabledRecipients()
	if err != nil {
		s.renderSettings(w, map[string]any{"Error": "Ошибка чтения получателей: " + err.Error()})
		return
	}
	if len(recipients) == 0 {
		s.renderSettings(w, map[string]any{"Error": "Нет активных получателей для тестовой отправки"})
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
		s.renderSettings(w, map[string]any{"Error": msg})
		return
	}
	s.renderSettings(w, map[string]any{"Success": "Тестовое сообщение отправлено всем активным получателям."})
}

func (s *Server) handleUploadSubmit(w http.ResponseWriter, r *http.Request) {
	// 32 MiB in-memory limit for parsed multipart parts; the file itself
	// spills to a temp file beyond that if larger.
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		s.renderSettings(w, map[string]any{"UploadError": "Не удалось прочитать форму: " + err.Error()})
		return
	}
	organization := strings.TrimSpace(r.FormValue("organization"))
	if organization == "" {
		s.renderSettings(w, map[string]any{"UploadError": "Укажите название организации, которой принадлежит файл"})
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		s.renderSettings(w, map[string]any{"UploadError": "Выберите CSV-файл для загрузки"})
		return
	}
	defer file.Close()

	result, err := csvimport.Import(s.store, file, organization)
	if err != nil {
		s.renderSettings(w, map[string]any{"UploadError": "Ошибка импорта: " + err.Error()})
		return
	}

	s.renderSettings(w, map[string]any{
		"UploadSuccess": "Импорт завершён: добавлено " + strconv.Itoa(result.Inserted) +
			", обновлено " + strconv.Itoa(result.Updated) +
			", пропущено " + strconv.Itoa(result.Skipped) + ".",
		"UploadErrors": result.Errors,
	})
}
