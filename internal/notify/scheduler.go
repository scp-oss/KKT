// Package notify runs the periodic check that notifies Telegram recipients
// about KKT/FN records approaching their expiry dates.
package notify

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"kkt-monitor/internal/db"
	"kkt-monitor/internal/telegram"
)

// Thresholds are the days-remaining values that trigger a notification.
var Thresholds = []int{30, 10, 5, 2, 1}

// maxSleep bounds how long Run ever sleeps in one stretch, so a change to
// the poll schedule in settings is picked up promptly instead of only after
// the previously configured time already passed.
const maxSleep = 15 * time.Minute

type Scheduler struct {
	store *db.DB
}

func New(store *db.DB) *Scheduler {
	return &Scheduler{store: store}
}

// Run blocks, checking for due notifications immediately and then at the
// configured poll times, until ctx is cancelled. Checking more often than
// scheduled is harmless: notification_log deduplicates by
// (kkt, field, threshold, date).
func (s *Scheduler) Run(ctx context.Context) {
	s.checkOnce(ctx)

	for {
		next := s.nextFireTime()
		wait := time.Until(next)
		if wait > maxSleep {
			wait = maxSleep
		}
		if wait < 0 {
			wait = 0
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
			if !time.Now().Before(next) {
				s.checkOnce(ctx)
			}
		}
	}
}

// nextFireTime reads the configured poll schedule (re-read every call, so
// changes in settings take effect without a restart) and returns the
// soonest upcoming HH:MM occurrence.
func (s *Scheduler) nextFireTime() time.Time {
	time1, time2, err := s.store.GetPollSchedule()
	if err != nil {
		log.Printf("notify: чтение расписания уведомлений: %v", err)
		time1, time2 = "09:00", ""
	}

	now := time.Now()
	next := nextOccurrence(now, time1)
	if time2 != "" {
		if t2 := nextOccurrence(now, time2); !t2.IsZero() && (next.IsZero() || t2.Before(next)) {
			next = t2
		}
	}
	if next.IsZero() {
		next = now.Add(time.Hour) // safety net for an unparsable schedule
	}
	return next
}

// nextOccurrence returns the next time "HH:MM" (server local time) occurs
// at or after now - today if it hasn't passed yet, tomorrow otherwise. A
// malformed value returns the zero Time.
func nextOccurrence(now time.Time, hhmm string) time.Time {
	hh, mm, ok := parseHHMM(hhmm)
	if !ok {
		return time.Time{}
	}
	candidate := time.Date(now.Year(), now.Month(), now.Day(), hh, mm, 0, 0, now.Location())
	if !candidate.After(now) {
		candidate = candidate.AddDate(0, 0, 1)
	}
	return candidate
}

func parseHHMM(s string) (hh, mm int, ok bool) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	hh, err1 := strconv.Atoi(parts[0])
	mm, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil || hh < 0 || hh > 23 || mm < 0 || mm > 59 {
		return 0, 0, false
	}
	return hh, mm, true
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

	anyRecipients, err := s.store.ListEnabledRecipients()
	if err != nil {
		log.Printf("notify: чтение получателей: %v", err)
		return
	}
	if len(anyRecipients) == 0 {
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
		recipients, err := s.store.ListEnabledRecipientsForOrganization(k.Organization)
		if err != nil {
			log.Printf("notify: чтение получателей для организации %q: %v", k.Organization, err)
			continue
		}
		if len(recipients) == 0 {
			continue
		}
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

	// due collects every threshold that's currently due (daysLeft <=
	// threshold) and not yet notified. Using <= rather than an exact ==
	// means a threshold that was never checked while precisely due - the
	// service was restarted, a poll was missed, or the record was only
	// just imported already inside the window - still gets caught here
	// instead of being silently skipped forever.
	var due []int
	for _, threshold := range Thresholds {
		if daysLeft > threshold {
			continue
		}
		already, err := s.store.WasNotified(k.ID, field, threshold, endDate)
		if err != nil {
			log.Printf("notify: проверка отправленных уведомлений: %v", err)
			return
		}
		if !already {
			due = append(due, threshold)
		}
	}
	if len(due) == 0 {
		return
	}

	// Several thresholds can end up due at once (a catch-up after
	// downtime, or a freshly imported record already close to expiry).
	// Sending one message per threshold would spam the same news several
	// times over, so send a single message - stating the real days
	// remaining, not a stale threshold number - and mark every due
	// threshold as handled, not just the one mentioned in the text.
	text := formatMessage(k, fieldLabel, endDate, daysLeft)
	errs := sender.SendToAll(ctx, recipients, text)
	for _, e := range errs {
		log.Printf("notify: ошибка отправки: %v", e)
	}
	// If every recipient failed (bot/relay unreachable, bad token, ...),
	// don't mark this as handled - leave it due so the next poll retries
	// automatically, instead of silently losing the notification because
	// of a transient outage. A partial failure (some recipients got it)
	// still marks as done: those who received it shouldn't get it again,
	// and the ones who didn't can be re-added as recipients if needed.
	if len(recipients) > 0 && len(errs) == len(recipients) {
		return
	}
	for _, threshold := range due {
		if err := s.store.MarkNotified(k.ID, field, threshold, endDate); err != nil {
			log.Printf("notify: сохранение отметки об отправке: %v", err)
		}
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
		"⚠ Внимание: %s\n\n%s истекает через %d %s (%s)\n\nОрганизация: %s\nККТ: %s\nЗав. номер ККТ: %s\nЗав. номер ФН: %s\nРег. номер ККТ: %s\nАдрес: %s",
		k.Model, fieldLabel, daysLeft, dayWord, endDate, k.Organization, k.Model, k.SerialNumber, k.FNNumber, k.RegNumber, k.Address,
	)
}
