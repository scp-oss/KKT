package db

import "time"

// KKT is one cash register (Контрольно-кассовая техника) record.
// SerialNumber (Заводской номер ККТ) is the unique key every record is keyed on.
type KKT struct {
	ID           int64
	Organization string // организация, к которой относится касса (одна на весь загруженный файл)
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
		_, err = d.Exec(`UPDATE kkt SET organization=?, reg_number=?, fn_number=?, address=?, model=?, ofd_end_date=?, fn_end_date=?, updated_at=? WHERE id=?`,
			k.Organization, k.RegNumber, k.FNNumber, k.Address, k.Model, k.OFDEndDate, k.FNEndDate, now, existingID)
		if err != nil {
			return 0, false, err
		}
		return existingID, false, nil
	default:
		res, insErr := d.Exec(`INSERT INTO kkt (serial_number, organization, reg_number, fn_number, address, model, ofd_end_date, fn_end_date, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			k.SerialNumber, k.Organization, k.RegNumber, k.FNNumber, k.Address, k.Model, k.OFDEndDate, k.FNEndDate, now, now)
		if insErr != nil {
			return 0, false, insErr
		}
		newID, _ := res.LastInsertId()
		return newID, true, nil
	}
}

// ListKKT returns every record ordered by the earliest of its two expiry
// dates (ОФД service end date, ФН end date) first; records with neither date
// set sort last.
func (d *DB) ListKKT() ([]KKT, error) {
	rows, err := d.Query(`SELECT id, serial_number, organization, reg_number, fn_number, address, model, ofd_end_date, fn_end_date, created_at, updated_at
		FROM kkt ORDER BY
		MIN(
			CASE WHEN ofd_end_date = '' THEN '9999-12-31' ELSE ofd_end_date END,
			CASE WHEN fn_end_date = '' THEN '9999-12-31' ELSE fn_end_date END
		) ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []KKT
	for rows.Next() {
		var k KKT
		if err := rows.Scan(&k.ID, &k.SerialNumber, &k.Organization, &k.RegNumber, &k.FNNumber, &k.Address, &k.Model, &k.OFDEndDate, &k.FNEndDate, &k.CreatedAt, &k.UpdatedAt); err != nil {
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

// ListOrganizations returns the distinct organization names present in the
// registry, sorted alphabetically. Used to populate the "notify for this
// organization" recipient dropdown.
func (d *DB) ListOrganizations() ([]string, error) {
	rows, err := d.Query(`SELECT DISTINCT organization FROM kkt WHERE organization != '' ORDER BY organization`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var org string
		if err := rows.Scan(&org); err != nil {
			return nil, err
		}
		out = append(out, org)
	}
	return out, rows.Err()
}
