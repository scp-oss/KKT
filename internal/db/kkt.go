package db

import "time"

// KKT is one cash register (Контрольно-кассовая техника) record.
// SerialNumber (Заводской номер ККТ) is the unique key every record is keyed on.
type KKT struct {
	ID           int64
	RegNumber    string // Регистрационный номер ККТ
	FNNumber     string // Заводской номер ФН
	SerialNumber string // Заводской номер ККТ (unique)
	Address      string // Адрес расчетов
	Model        string // Модель ККТ
	OFDEndDate   string // Дата окончания оказания услуг (ОФД), ISO yyyy-mm-dd or ""
	FNEndDate    string // Дата окончания срока ФН, ISO yyyy-mm-dd or ""
	CreatedAt    string
	UpdatedAt    string
}

// UpsertKKT inserts a new record or updates the existing one matched by SerialNumber.
// It returns the record id and whether a new row was inserted.
func (d *DB) UpsertKKT(k KKT) (id int64, inserted bool, err error) {
	now := time.Now().UTC().Format(time.RFC3339)

	var existingID int64
	err = d.QueryRow(`SELECT id FROM kkt WHERE serial_number = ?`, k.SerialNumber).Scan(&existingID)
	switch err {
	case nil:
		_, err = d.Exec(`UPDATE kkt SET reg_number=?, fn_number=?, address=?, model=?, ofd_end_date=?, fn_end_date=?, updated_at=? WHERE id=?`,
			k.RegNumber, k.FNNumber, k.Address, k.Model, k.OFDEndDate, k.FNEndDate, now, existingID)
		if err != nil {
			return 0, false, err
		}
		return existingID, false, nil
	default:
		res, insErr := d.Exec(`INSERT INTO kkt (serial_number, reg_number, fn_number, address, model, ofd_end_date, fn_end_date, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			k.SerialNumber, k.RegNumber, k.FNNumber, k.Address, k.Model, k.OFDEndDate, k.FNEndDate, now, now)
		if insErr != nil {
			return 0, false, insErr
		}
		newID, _ := res.LastInsertId()
		return newID, true, nil
	}
}

func (d *DB) ListKKT() ([]KKT, error) {
	rows, err := d.Query(`SELECT id, serial_number, reg_number, fn_number, address, model, ofd_end_date, fn_end_date, created_at, updated_at
		FROM kkt ORDER BY
		CASE WHEN ofd_end_date = '' THEN 1 ELSE 0 END, ofd_end_date ASC,
		CASE WHEN fn_end_date = '' THEN 1 ELSE 0 END, fn_end_date ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []KKT
	for rows.Next() {
		var k KKT
		if err := rows.Scan(&k.ID, &k.SerialNumber, &k.RegNumber, &k.FNNumber, &k.Address, &k.Model, &k.OFDEndDate, &k.FNEndDate, &k.CreatedAt, &k.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

func (d *DB) DeleteKKT(id int64) error {
	_, err := d.Exec(`DELETE FROM kkt WHERE id = ?`, id)
	return err
}

func (d *DB) CountKKT() (int, error) {
	var n int
	err := d.QueryRow(`SELECT COUNT(*) FROM kkt`).Scan(&n)
	return n, err
}
