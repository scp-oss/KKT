package db

import "time"

// WasNotified reports whether a notification for this KKT/field/threshold/end-date
// combination was already sent (so the scheduler can safely run more than once a day).
func (d *DB) WasNotified(kktID int64, field string, thresholdDays int, endDate string) (bool, error) {
	var n int
	err := d.QueryRow(`SELECT COUNT(*) FROM notification_log WHERE kkt_id=? AND field=? AND threshold_days=? AND end_date=?`,
		kktID, field, thresholdDays, endDate).Scan(&n)
	return n > 0, err
}

func (d *DB) MarkNotified(kktID int64, field string, thresholdDays int, endDate string) error {
	_, err := d.Exec(`INSERT OR IGNORE INTO notification_log (kkt_id, field, threshold_days, end_date, sent_at) VALUES (?, ?, ?, ?, ?)`,
		kktID, field, thresholdDays, endDate, time.Now().UTC().Format(time.RFC3339))
	return err
}
