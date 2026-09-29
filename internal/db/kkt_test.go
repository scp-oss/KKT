package db

import "testing"

func newTestDB(t *testing.T) *DB {
	t.Helper()
	store, err := Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestUpsertKKTDoesNotBlankExistingFields(t *testing.T) {
	store := newTestDB(t)

	_, inserted, err := store.UpsertKKT(KKT{
		SerialNumber: "SN1",
		Organization: "ИП Пупкин И.В.",
		Address:      "г Пенза, ул Ленина, д. 1", // fixed by hand after an earlier import left it blank
		Model:        "Пирит Лайт Ф",
		OFDEndDate:   "2027-01-01",
	})
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	if !inserted {
		t.Fatalf("first upsert: expected insert")
	}

	// Re-uploading the CSV: this row's address can't be parsed again (blank),
	// but the model changed and a new FN end date appeared.
	_, inserted, err = store.UpsertKKT(KKT{
		SerialNumber: "SN1",
		Organization: "ИП Пупкин И.В.",
		Address:      "",
		Model:        "Пирит Лайт Ф2",
		OFDEndDate:   "2027-01-01",
		FNEndDate:    "2027-06-01",
	})
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	if inserted {
		t.Fatalf("second upsert: expected update, got insert")
	}

	records, err := store.ListKKT()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	rec := records[0]

	if rec.Address != "г Пенза, ул Ленина, д. 1" {
		t.Errorf("Address = %q, want the manually-fixed value to survive a blank re-import", rec.Address)
	}
	if rec.Model != "Пирит Лайт Ф2" {
		t.Errorf("Model = %q, want the new non-blank value to have been applied", rec.Model)
	}
	if rec.FNEndDate != "2027-06-01" {
		t.Errorf("FNEndDate = %q, want the new non-blank value to have been applied", rec.FNEndDate)
	}
}

func TestUpdateLicenseEndDate(t *testing.T) {
	store := newTestDB(t)

	if _, _, err := store.UpsertKKT(KKT{SerialNumber: "SN1", Organization: "ИП Пупкин И.В."}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	matched, err := store.UpdateLicenseEndDate("SN1", "2027-07-06")
	if err != nil {
		t.Fatalf("UpdateLicenseEndDate: %v", err)
	}
	if !matched {
		t.Errorf("expected SN1 to be matched")
	}

	records, err := store.ListKKT()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].LicenseEndDate != "2027-07-06" {
		t.Errorf("LicenseEndDate = %q, want 2027-07-06", records[0].LicenseEndDate)
	}

	matched, err = store.UpdateLicenseEndDate("DOES-NOT-EXIST", "2027-07-06")
	if err != nil {
		t.Fatalf("UpdateLicenseEndDate(unmatched): %v", err)
	}
	if matched {
		t.Errorf("expected no match for a serial number that doesn't exist")
	}
}

func TestListKKTOrdersUnactivatedFirst(t *testing.T) {
	store := newTestDB(t)

	// Expires very soon - would normally sort first under a plain
	// earliest-date ordering.
	if _, _, err := store.UpsertKKT(KKT{SerialNumber: "SOON", OFDEndDate: "2027-01-01", FNEndDate: "2027-01-01"}); err != nil {
		t.Fatalf("upsert SOON: %v", err)
	}
	// No ОФД subscription at all - must outrank SOON regardless.
	if _, _, err := store.UpsertKKT(KKT{SerialNumber: "UNACTIVATED", OFDEndDate: "", FNEndDate: "2027-12-31"}); err != nil {
		t.Fatalf("upsert UNACTIVATED: %v", err)
	}
	// Expires much later - should sort last.
	if _, _, err := store.UpsertKKT(KKT{SerialNumber: "LATER", OFDEndDate: "2028-01-01", FNEndDate: "2028-01-01"}); err != nil {
		t.Fatalf("upsert LATER: %v", err)
	}

	records, err := store.ListKKT()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("expected 3 records, got %d", len(records))
	}
	got := []string{records[0].SerialNumber, records[1].SerialNumber, records[2].SerialNumber}
	want := []string{"UNACTIVATED", "SOON", "LATER"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("order = %v, want %v", got, want)
			break
		}
	}
}
