// Package csvimport parses the "Мониторинг ККТ и ФН" CSV export and upserts
// records into the database. The export is a semicolon-separated Excel CSV
// where every cell is wrapped as ="value" to force text formatting.
package csvimport

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"strings"
	"time"

	"kkt-monitor/internal/db"
)

// column headers we care about, matched by exact name against the file's header row.
const (
	colRegNumber  = "Регистрационный номер ККТ"
	colFNNumber   = "Заводской номер ФН"
	colSerial     = "Заводской номер ККТ"
	colAddress    = "Адрес расчетов"
	colOFDEndDate = "Дата окончания оказания услуг"
	colModel      = "Модель ККТ"
	colFNEndDate  = "Дата окончания срока ФН"
)

var requiredColumns = []string{colRegNumber, colFNNumber, colSerial, colAddress, colOFDEndDate, colModel, colFNEndDate}

type Result struct {
	Inserted int
	Updated  int
	Skipped  int
	Errors   []string
}

// Import reads a CSV export from r and upserts every row into the database,
// keyed on the unique "Заводской номер ККТ" (serial number) column.
func Import(store *db.DB, r io.Reader) (Result, error) {
	var res Result

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
	for _, col := range requiredColumns {
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

		serial := get(record, index, colSerial)
		if serial == "" {
			res.Skipped++
			res.Errors = append(res.Errors, fmt.Sprintf("строка %d: пропущена, не указан заводской номер ККТ", rowNum))
			continue
		}

		rec := db.KKT{
			SerialNumber: serial,
			RegNumber:    get(record, index, colRegNumber),
			FNNumber:     get(record, index, colFNNumber),
			Address:      get(record, index, colAddress),
			Model:        get(record, index, colModel),
			OFDEndDate:   parseDate(get(record, index, colOFDEndDate)),
			FNEndDate:    parseDate(get(record, index, colFNEndDate)),
		}

		_, inserted, err := store.UpsertKKT(rec)
		if err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("строка %d (%s): %v", rowNum, serial, err))
			continue
		}
		if inserted {
			res.Inserted++
		} else {
			res.Updated++
		}
	}

	return res, nil
}

func get(record []string, index map[string]int, col string) string {
	i, ok := index[col]
	if !ok || i >= len(record) {
		return ""
	}
	return strings.TrimSpace(record[i])
}

func isBlankRow(record []string) bool {
	for _, f := range record {
		if strings.TrimSpace(f) != "" {
			return false
		}
	}
	return true
}

// unwrapCell strips the Excel ="..." text-forcing wrapper, if present.
func unwrapCell(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, `="`) && strings.HasSuffix(s, `"`) && len(s) >= 3 {
		return s[2 : len(s)-1]
	}
	return strings.Trim(s, `"`)
}

var dateLayouts = []string{
	"2006-01-02",
	"2006-01-02T15:04:05",
	"02.01.2006 15:04:05",
	"02.01.2006 15:04",
	"02.01.2006",
}

// parseDate normalizes the various date/time formats seen in the export to
// a plain ISO date (yyyy-mm-dd). Unparseable or empty values return "".
func parseDate(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	for _, layout := range dateLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Format("2006-01-02")
		}
	}
	return ""
}

func stripBOM(r io.Reader) io.Reader {
	br := bufio.NewReader(r)
	bom, err := br.Peek(3)
	if err == nil && len(bom) == 3 && bom[0] == 0xEF && bom[1] == 0xBB && bom[2] == 0xBF {
		br.Discard(3)
	}
	return br
}
