// Package csvimport parses the "Мониторинг ККТ и ФН" CSV export and upserts
// records into the database. The export is a semicolon-separated Excel CSV
// where every cell is wrapped as ="value" to force text formatting.
package csvimport

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"regexp"
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
// keyed on the unique "Заводской номер ККТ" (serial number) column. Every
// record in the file is tagged with organization, since one file always
// belongs to a single organization.
func Import(store *db.DB, r io.Reader, organization string) (Result, error) {
	var res Result
	organization = strings.TrimSpace(organization)

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
			Organization: organization,
			RegNumber:    get(record, index, colRegNumber),
			FNNumber:     get(record, index, colFNNumber),
			Address:      normalizeAddress(get(record, index, colAddress)),
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

// normalizeAddress strips the region/postal-code/administrative-district
// cruft that the source export prefixes addresses with, leaving just the
// settlement, street and house, e.g.:
//
//	58 - Пензенская область, 440072, г Пенза,     ул Антонова, стр 18В
//	-> г Пенза, ул Антонова, стр 18В
//
//	440528, Пензенская обл. с. Богословка, ул. Дорожная, д. 1
//	-> с. Богословка, ул. Дорожная, д. 1
var (
	reWhitespace   = regexp.MustCompile(`\s+`)
	reLeadingCode  = regexp.MustCompile(`^\d+\s*-\s*`)
	reCountry      = regexp.MustCompile(`(?i)россия,?\s*`)
	rePostalCode   = regexp.MustCompile(`\b\d{6}\b,?\s*`)
	reMunicipal    = regexp.MustCompile(`(?i)г\.?\s*о\.?\s*город\s+[А-ЯЁа-яё-]+,?\s*`)
	reMunicipalRev = regexp.MustCompile(`(?i)город\s+([А-ЯЁ][А-ЯЁа-яё-]*)\s*г\.?\s*о\.?,?\s*`)
	reDistrict     = regexp.MustCompile(`(?i)(муниципальный\s+район|м\.\s*р-н|сельское\s+поселение|городской\s+округ|р-н)\s*[А-ЯЁа-яё-]*,?\s*`)
	// Region name given as "<Name> <keyword>", covering every Russian federal
	// subject type that uses a trailing qualifier: областная/край/республика/
	// автономный округ (including double-barrel names like "Ханты-Мансийский
	// автономный округ - Югра").
	// "об." (dot mandatory) is included alongside "обл." - some exports
	// abbreviate область that far; the mandatory dot keeps it from matching
	// inside unrelated words.
	reRegionSuffix = regexp.MustCompile(`(?i)[А-ЯЁа-яё-]+(?:\s*-\s*[А-ЯЁа-яё]+)?\s*(область|обл\.?|об\.|край|республика|респ\.?|автономная область|автономный округ|авт\.?\s*округ|АО)(?:\s*-\s*[А-ЯЁа-яё]+)?,?\s*`)
	// Region name given as "<keyword> <Name>" (e.g. "Республика Татарстан").
	reRegionPrefix = regexp.MustCompile(`(?i)(республика|автономная\s+область|автономный\s+округ)\s+[А-ЯЁ][А-ЯЁа-яё-]*(?:\s*-\s*[А-ЯЁа-яё]+)?,?\s*`)
	reGluedCity    = regexp.MustCompile(`^г([А-ЯЁ])`)
	// a short bare number left dangling at the start after other cleanup is a
	// leftover RF region code (e.g. "58" for Пензенская область), never a
	// real address component.
	reLeadingBareCode = regexp.MustCompile(`^\d{1,3}\s*,\s*`)
	// Matches a city name immediately followed by a second mention of the
	// same city marked with a trailing "г."/"город" (as in the malformed
	// "г Пенза, Пенза г.," pattern seen in some export rows). Deliberately
	// not case-insensitive and requires the mandatory trailing "г." marker,
	// so it never mistakes a lowercase street abbreviation (e.g. "ул") for a
	// repeated city name.
	reDuplicateCity = regexp.MustCompile(`^(г\.?\s*[А-ЯЁ][а-яё-]+),\s*(?:город\.?\s*|г\.?\s*)?[А-ЯЁ][а-яё-]+\s*г\.?,?\s*`)
	// "ст р." is "стр." (строение) with a stray space splitting it, seen in
	// some export rows. \b doesn't work here - Go's regexp only treats ASCII
	// letters as word characters, so it never fires before/after Cyrillic -
	// the preceding delimiter is captured and put back instead.
	reSplitStroenie = regexp.MustCompile(`(^|[,\s])ст\s+р\.`)
	reMultiComma    = regexp.MustCompile(`\s*,(?:\s*,)+\s*`)
	reLeadingJunk   = regexp.MustCompile(`^[,\s]+`)
	reTrailingJunk  = regexp.MustCompile(`[,\s]+$`)
)

func normalizeAddress(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	s = reWhitespace.ReplaceAllString(s, " ")
	s = reLeadingCode.ReplaceAllString(s, "")
	s = reCountry.ReplaceAllString(s, "")
	s = reMunicipal.ReplaceAllString(s, "")
	s = reMunicipalRev.ReplaceAllString(s, "г $1, ")
	s = reDistrict.ReplaceAllString(s, "")
	s = rePostalCode.ReplaceAllString(s, "")
	s = reRegionPrefix.ReplaceAllString(s, "")
	s = reRegionSuffix.ReplaceAllString(s, "")
	s = reGluedCity.ReplaceAllString(s, "г $1")
	for i := 0; i < 2; i++ {
		trimmed := reLeadingBareCode.ReplaceAllString(s, "")
		if trimmed == s {
			break
		}
		s = trimmed
	}
	s = reDuplicateCity.ReplaceAllString(s, "$1, ")
	s = reSplitStroenie.ReplaceAllString(s, "${1}стр.")
	s = reMultiComma.ReplaceAllString(s, ", ")
	s = reLeadingJunk.ReplaceAllString(s, "")
	s = reTrailingJunk.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

func stripBOM(r io.Reader) io.Reader {
	br := bufio.NewReader(r)
	bom, err := br.Peek(3)
	if err == nil && len(bom) == 3 && bom[0] == 0xEF && bom[1] == 0xBB && bom[2] == 0xBF {
		br.Discard(3)
	}
	return br
}
