package notify

import (
	"context"
	"testing"
	"time"

	"kkt-monitor/internal/db"
	"kkt-monitor/internal/telegram"
)

func newTestDB(t *testing.T) *db.DB {
	t.Helper()
	store, err := db.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

// TestCheckFieldCatchesUpMissedThreshold covers the bug report: a threshold
// that was exactly due while the service happened not to check (a restart,
// downtime, a missed poll) used to be skipped forever, because the old
// comparison only matched daysLeft == threshold exactly. daysLeft here jumps
// straight from "not due yet" to 3 days left, skipping over the 5-day mark
// entirely - it must still fire the 5-day notification (with the real
// day count in the text), not silently drop it.
func TestCheckFieldCatchesUpMissedThreshold(t *testing.T) {
	store := newTestDB(t)
	sender := telegram.New(db.BotSettings{})
	today := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)
	endDate := today.AddDate(0, 0, 3).Format("2006-01-02") // daysLeft = 3

	k := db.KKT{ID: 1}
	s := &Scheduler{store: store}

	s.checkField(context.Background(), sender, nil, k, "fn", "ФН", endDate, today)

	notified5, err := store.WasNotified(k.ID, "fn", 5, endDate)
	if err != nil {
		t.Fatalf("WasNotified(5): %v", err)
	}
	if !notified5 {
		t.Errorf("expected the missed 5-day threshold to be caught up, but it wasn't marked notified")
	}
	notified2, err := store.WasNotified(k.ID, "fn", 2, endDate)
	if err != nil {
		t.Fatalf("WasNotified(2): %v", err)
	}
	if notified2 {
		t.Errorf("threshold 2 isn't due yet (daysLeft=3), it should not have been marked")
	}

	// The larger thresholds (30, 10) were also skipped over by the same
	// jump straight to daysLeft=3 - a single message covers all of them,
	// so they must be marked too or they'd each fire their own message on
	// a later check.
	for _, threshold := range []int{30, 10} {
		notified, err := store.WasNotified(k.ID, "fn", threshold, endDate)
		if err != nil {
			t.Fatalf("WasNotified(%d): %v", threshold, err)
		}
		if !notified {
			t.Errorf("threshold %d was also skipped over and should have been superseded by the single catch-up message", threshold)
		}
	}

	// A second check the same day must not re-send anything.
	s.checkField(context.Background(), sender, nil, k, "fn", "ФН", endDate, today)
	var n int
	if err := store.QueryRow(`SELECT COUNT(*) FROM notification_log WHERE kkt_id=? AND field=? AND end_date=?`,
		k.ID, "fn", endDate).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 3 {
		t.Errorf("notification_log rows = %d, want exactly 3 (30, 10, 5 - no duplicate send)", n)
	}

	// A day later daysLeft drops to 2 - now the 2-day threshold becomes due.
	tomorrow := today.AddDate(0, 0, 1)
	s.checkField(context.Background(), sender, nil, k, "fn", "ФН", endDate, tomorrow)
	notified2, err = store.WasNotified(k.ID, "fn", 2, endDate)
	if err != nil {
		t.Fatalf("WasNotified(2) after day advances: %v", err)
	}
	if !notified2 {
		t.Errorf("expected threshold 2 to fire once daysLeft dropped to 2")
	}
}

// TestCheckFieldRetriesOnTotalSendFailure covers a second way a notification
// used to go missing silently: if the Telegram send failed for every single
// recipient (bot unreachable, bad token, relay down), the threshold was
// still marked as handled and never retried. A bad db.BotSettings.Token
// makes every send fail before any network call, standing in for that kind
// of outage.
func TestCheckFieldRetriesOnTotalSendFailure(t *testing.T) {
	store := newTestDB(t)
	failingSender := telegram.New(db.BotSettings{}) // no token configured -> every Send fails
	today := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)
	endDate := today.AddDate(0, 0, 5).Format("2006-01-02") // daysLeft = 5
	recipients := []db.Recipient{{ChatID: "12345"}}

	k := db.KKT{ID: 1}
	s := &Scheduler{store: store}

	s.checkField(context.Background(), failingSender, recipients, k, "fn", "ФН", endDate, today)

	notified, err := store.WasNotified(k.ID, "fn", 5, endDate)
	if err != nil {
		t.Fatalf("WasNotified(5): %v", err)
	}
	if notified {
		t.Errorf("every recipient failed to receive it - threshold 5 must stay due for a retry, not be marked handled")
	}
}
