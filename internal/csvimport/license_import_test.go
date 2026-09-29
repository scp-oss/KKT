package csvimport

import (
	"strings"
	"testing"

	"kkt-monitor/internal/db"
)

// licenseSample mirrors the real "ТС ПиОТ" export: semicolon-delimited,
// double-quoted fields, with a bare leading apostrophe (not the ="..."
// wrapper the main export uses) forcing text formatting on numeric IDs.
const licenseSample = `"Модель";"Наименование кассы";"Регистрационный номер";"Номер фискального накопителя";"Срок окончания фискального накопителя";"Заводской номер";"Адрес";"Дата активации лицензии";"Дата окончания лицензии";"Версия ЕСМ";"Кассовый софт"` + "\r\n" +
	`"Пирит Лайт Ф";"Моя касса";"'0010319997051316";"'7384440901528822";"";"'0130001113";"г. Пенза, ул. Коннозаводская, 25 а";"06.07.2026";"06.07.2027";"1.6.4.0";"unknown"` + "\r\n" +
	`"Пирит Лайт Ф";"Моя касса";"'0010313846018587";"'7384440901529008";"";"'0130009999";"г. Пенза, ул. Мира, 44A #1";"11.08.2026";"11.08.2027";"1.6.4.0";"unknown"` + "\r\n"

func TestImportLicensesMatchesBySerialAndSkipsUnmatched(t *testing.T) {
	store := newTestDB(t)

	// Pre-seed one record via the main import path, keyed on the serial
	// number the license export also carries in its "Заводской номер"
	// column - this is the record the license file should enrich.
	if _, _, err := store.UpsertKKT(db.KKT{
		SerialNumber: "0130001113",
		Organization: "ИП Пупкин И.В.",
		Model:        "Пирит Лайт Ф",
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	res, err := ImportLicenses(store, strings.NewReader(licenseSample))
	if err != nil {
		t.Fatalf("ImportLicenses: %v", err)
	}
	if res.Updated != 1 {
		t.Errorf("Updated = %d, want 1", res.Updated)
	}
	if len(res.NotFound) != 1 || !strings.Contains(res.NotFound[0], "0130009999") {
		t.Errorf("NotFound = %v, want exactly one entry for the unmatched serial 0130009999", res.NotFound)
	}
	if len(res.Errors) != 0 {
		t.Errorf("Errors = %v, want none", res.Errors)
	}

	records, err := store.ListKKT()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected the unmatched row to not create a new record, got %d records", len(records))
	}
	if records[0].LicenseEndDate != "2027-07-06" {
		t.Errorf("LicenseEndDate = %q, want 2027-07-06 (parsed from 06.07.2027)", records[0].LicenseEndDate)
	}
	// Fields outside the license file's scope must be untouched.
	if records[0].Organization != "ИП Пупкин И.В." {
		t.Errorf("Organization = %q, want it left untouched by the license-only import", records[0].Organization)
	}
}

func TestImportLicensesMissingRequiredColumn(t *testing.T) {
	store := newTestDB(t)
	bad := `"Модель";"Заводской номер"` + "\r\n" + `"Пирит";"'0130001113"` + "\r\n"
	if _, err := ImportLicenses(store, strings.NewReader(bad)); err == nil {
		t.Fatal("expected an error for a missing \"Дата окончания лицензии\" column")
	}
}
