package csvimport

import (
	"encoding/csv"
	"fmt"
	"io"

	"kkt-monitor/internal/db"
)

// Column headers for the separate "ТС ПиОТ" export, which tracks the cash
// software license rather than ОФД/ФН - a different source system, but keyed
// on the same "Заводской номер" (ККТ serial number) this app already uses as
// its unique key. Address and model aren't required: they only make the
// "not found" report below easier to act on.
const (
	licenseColSerial  = "Заводской номер"
	licenseColEndDate = "Дата окончания лицензии"
	licenseColModel   = "Модель"
	licenseColAddress = "Адрес"
)

var licenseRequiredColumns = []string{licenseColSerial, licenseColEndDate}

type LicenseResult struct {
	Updated int
	// NotFound lists rows whose serial number doesn't match any existing
	// KKT record - this import only enriches records that came from the
	// main "Мониторинг ККТ и ФН" upload, it never creates new ones (the
	// ПиОТ export has no organization, which every record here requires).
	NotFound []string
	Skipped  int
	Errors   []string
}

// ImportLicenses reads the "ТС ПиОТ" CSV export and updates the license
// expiry date of every matching KKT record, identified by serial number.
// Records not already in the registry are reported, not created.
func ImportLicenses(store *db.DB, r io.Reader) (LicenseResult, error) {
	var res LicenseResult

	reader := csv.NewReader(stripBOM(r))
	reader.Comma = ';'
	reader.LazyQuotes = true
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true

	header, err := reader.Read()
	if err != nil {
		return res, fmt.Errorf("чтение заголовка CSV: %w", err)
	}
	for i := range header {
		header[i] = unwrapCell(header[i])
	}

	index := map[string]int{}
	for i, name := range header {
		index[name] = i
	}
	for _, col := range licenseRequiredColumns {
		if _, ok := index[col]; !ok {
			return res, fmt.Errorf("в файле отсутствует колонка %q", col)
		}
	}

	rowNum := 1
	for {
		rowNum++
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("строка %d: %v", rowNum, err))
			continue
		}
		if isBlankRow(record) {
			continue
		}
		for i := range record {
			record[i] = unwrapCell(record[i])
		}

		serial := get(record, index, licenseColSerial)
		if serial == "" {
			res.Skipped++
			res.Errors = append(res.Errors, fmt.Sprintf("строка %d: пропущена, не указан заводской номер", rowNum))
			continue
		}

		rawEndDate := get(record, index, licenseColEndDate)
		endDate := parseDate(rawEndDate)
		if rawEndDate != "" && endDate == "" {
			res.Skipped++
			res.Errors = append(res.Errors, fmt.Sprintf("строка %d (%s): не удалось разобрать дату окончания лицензии %q", rowNum, serial, rawEndDate))
			continue
		}

		matched, err := store.UpdateLicenseEndDate(serial, endDate)
		if err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("строка %d (%s): %v", rowNum, serial, err))
			continue
		}
		if !matched {
			label := serial
			if model := get(record, index, licenseColModel); model != "" {
				label += ", " + model
			}
			if addr := get(record, index, licenseColAddress); addr != "" {
				label += ", " + addr
			}
			res.NotFound = append(res.NotFound, label)
			continue
		}
		res.Updated++
	}

	return res, nil
}
