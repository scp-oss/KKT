// Package notify runs the periodic check that notifies Telegram recipients
// about KKT/FN records approaching their expiry dates.
package notify

import (
	"context"
	"fmt"
	"log"
	"time"

	"kkt-monitor/internal/db"
	"kkt-monitor/internal/telegram"
)

// Thresholds are the days-remaining values that trigger a notification.
var Thresholds = []int{30, 10, 5, 2, 1}

const checkInterval = time.Hour

type Scheduler struct {
	store *db.DB
}

func New(store *db.DB) *Scheduler {
	return &Scheduler{store: store}
}

// Run blocks, checking for due notifications immediately and then on a
// fixed interval, until ctx is cancelled. Checking more often than daily is
// harmless: notification_log deduplicates by (kkt, field, threshold, date).
func (s *Scheduler) Run(ctx context.Context) {
	s.checkOnce(ctx)

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.checkOnce(ctx)
		}
	}
}

func (s *Scheduler) checkOnce(ctx context.Context) {
	if err := s.store.PruneExpiredSessions(); err != nil {
		log.Printf("notify: очистка сессий: %v", err)
	}

	settings, err := s.store.GetBotSettings()
	if err != nil {
		log.Printf("notify: чтение настроек бота: %v", err)
		return
	}
	if settings.Token == "" {
		return // bot not configured yet
	}

	recipients, err := s.store.ListEnabledRecipients()
	if err != nil {
		log.Printf("notify: чтение получателей: %v", err)
		return
	}
	if len(recipients) == 0 {
		return
	}

	records, err := s.store.ListKKT()
	if err != nil {
		log.Printf("notify: чтение списка ККТ: %v", err)
		return
	}

	sender := telegram.New(settings)
	today := time.Now().Truncate(24 * time.Hour)

	for _, k := range records {
		s.checkField(ctx, sender, recipients, k, "ofd", "Дата окончания оказания услуг (ОФД)", k.OFDEndDate, today)
		s.checkField(ctx, sender, recipients, k, "fn", "Дата окончания срока ФН", k.FNEndDate, today)
	}
}

func (s *Scheduler) checkField(ctx context.Context, sender *telegram.Sender, recipients []db.Recipient, k db.KKT, field, fieldLabel, endDate string, today time.Time) {
	if endDate == "" {
		return
	}
	end, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		return
	}
	daysLeft := int(end.Sub(today).Hours() / 24)

	for _, threshold := range Thresholds {
		if daysLeft != threshold {
			continue
		}
		already, err := s.store.WasNotified(k.ID, field, threshold, endDate)
		if err != nil {
			log.Printf("notify: проверка отправленных уведомлений: %v", err)
			return
		}
		if already {
			return
		}

		text := formatMessage(k, fieldLabel, endDate, threshold)
		errs := sender.SendToAll(ctx, recipients, text)
		for _, e := range errs {
			log.Printf("notify: ошибка отправки: %v", e)
		}
		// Mark as sent even on partial failure so we don't spam retries every
		// hour; the next threshold crossing will try again.
		if err := s.store.MarkNotified(k.ID, field, threshold, endDate); err != nil {
			log.Printf("notify: сохранение отметки об отправке: %v", err)
		}
		return
	}
}

func formatMessage(k db.KKT, fieldLabel, endDate string, daysLeft int) string {
	dayWord := "дней"
	switch {
	case daysLeft == 1:
		dayWord = "день"
	case daysLeft >= 2 && daysLeft <= 4:
		dayWord = "дня"
	}
	return fmt.Sprintf(
		"⚠ Внимание: %s\n\n%s истекает через %d %s (%s)\n\nККТ: %s\nЗав. номер ККТ: %s\nЗав. номер ФН: %s\nРег. номер ККТ: %s\nАдрес: %s",
		k.Model, fieldLabel, daysLeft, dayWord, endDate, k.Model, k.SerialNumber, k.FNNumber, k.RegNumber, k.Address,
	)
}
