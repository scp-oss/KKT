package csvimport

import (
	"strings"
	"testing"

	"kkt-monitor/internal/db"
)

// sample mirrors the real "Мониторинг ККТ и ФН" export format: semicolon
// delimited, every cell wrapped as ="value", with mixed date formats and a
// UTF-8 BOM at the start.
const sample = "\uFEFF" +
	`="Регистрационный номер ККТ";="Заводской номер ФН";="Заводской номер ККТ";="Статус регистрации в ФНС";="Дата последнего документа";="Название ККТ";="Адрес расчетов";="Место расчетов";="Путь ККТ в папке Мои кассы";="Id ККТ";="Дата получения отчета о регистрации/перерегистрации";="Дата формирования отчета о регистрации/перерегистрации";="Дата окончания оказания услуг";="Постоплатный тариф";="Количество фискальных документов, полученных ОФД";="Модель ККТ";="Последнее закрытие смены";="Состояние смены";="Дата окончания срока ФН";="Количество фискальных документов в ФН";="Заполненность ФН";="ФФД";="Дата прогноза заполнения ФН";="Текущий тариф";="Остаток по тарифу (количество чеков/количество дней)";="Количество кодов активации в очереди (которые применены, но еще не начали действовать)";="Дата применения последнего кода активации"` + "\r\n" +
	`="0010399223014805";="7380440903712738";="0130002204";="Доступна перерегистрация и снятие с учета";="10.09.2026 22:00:00";;="г Пенза, ул Бийская, д. 7";="Магазин";="/Все кассы";="bb6662a4-83ab-4d1c-8c1a-d34a01f86ece";="17.07.2026 10:34:00";="17.07.2026 7:34:00";="2027-07-23";="Нет";="3282";="Пирит Лайт Ф";="10.09.2026 22:00:00";="Закрыта";="31.08.2027 10:34:00";="3282";="1 %";="1.2";="31.08.2027 10:34:00";="ОФД Красный 12 мес безлимит";="317";="0";="24.07.2026 6:18:00"` + "\r\n" +
	`="0010534348042942";="7384441001649905";="0130000213";="Доступна перерегистрация и снятие с учета";="10.09.2026 22:01:00";;="г. Пенза, ул. Бородина, д. 2";="Магазин";="/Все кассы";="a5624dc8-9bb8-436d-88e3-72cfa3acb056";="08.09.2026 9:21:00";="08.09.2026 6:21:00";;="Нет";="119";="Пирит Лайт Ф";="10.09.2026 22:01:00";="Закрыта";="23.10.2027 9:21:00";="119";="0 %";="1.2";="23.10.2027 9:21:00";;="0";="0";` + "\r\n\r\n"

func newTestDB(t *testing.T) *db.DB {
	t.Helper()
	store, err := db.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestImport(t *testing.T) {
	store := newTestDB(t)

	res, err := Import(store, strings.NewReader(sample), "ИП Пупкин И.В.")
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if res.Inserted != 2 || res.Updated != 0 || res.Skipped != 0 {
		t.Fatalf("got inserted=%d updated=%d skipped=%d errors=%v, want inserted=2 updated=0 skipped=0", res.Inserted, res.Updated, res.Skipped, res.Errors)
	}

	records, err := store.ListKKT()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}

	var withOFD, withoutOFD *db.KKT
	for i := range records {
		if records[i].SerialNumber == "0130002204" {
			withOFD = &records[i]
		}
		if records[i].SerialNumber == "0130000213" {
			withoutOFD = &records[i]
		}
	}
	if withOFD == nil || withoutOFD == nil {
		t.Fatalf("expected both records to import")
	}

	if withOFD.RegNumber != "0010399223014805" {
		t.Errorf("RegNumber = %q, want 0010399223014805", withOFD.RegNumber)
	}
	if withOFD.FNNumber != "7380440903712738" {
		t.Errorf("FNNumber = %q, want 7380440903712738", withOFD.FNNumber)
	}
	if withOFD.Model != "Пирит Лайт Ф" {
		t.Errorf("Model = %q, want Пирит Лайт Ф", withOFD.Model)
	}
	// ISO-formatted source column parses straight through.
	if withOFD.OFDEndDate != "2027-07-23" {
		t.Errorf("OFDEndDate = %q, want 2027-07-23", withOFD.OFDEndDate)
	}
	// dd.mm.yyyy hh:mm:ss source column normalizes to ISO.
	if withOFD.FNEndDate != "2027-08-31" {
		t.Errorf("FNEndDate = %q, want 2027-08-31", withOFD.FNEndDate)
	}
	if withOFD.Address != "г Пенза, ул Бийская, д. 7" {
		t.Errorf("Address = %q", withOFD.Address)
	}
	if withOFD.Organization != "ИП Пупкин И.В." {
		t.Errorf("Organization = %q, want ИП Пупкин И.В.", withOFD.Organization)
	}

	// empty source cell for the OFD end date column must stay empty, not error.
	if withoutOFD.OFDEndDate != "" {
		t.Errorf("OFDEndDate = %q, want empty", withoutOFD.OFDEndDate)
	}
	if withoutOFD.FNEndDate != "2027-10-23" {
		t.Errorf("FNEndDate = %q, want 2027-10-23", withoutOFD.FNEndDate)
	}
}

func TestImportUpsertsBySerialNumber(t *testing.T) {
	store := newTestDB(t)

	if _, err := Import(store, strings.NewReader(sample), "ИП Пупкин И.В."); err != nil {
		t.Fatalf("first import: %v", err)
	}
	res, err := Import(store, strings.NewReader(sample), "ИП Пупкин И.В.")
	if err != nil {
		t.Fatalf("second import: %v", err)
	}
	if res.Inserted != 0 || res.Updated != 2 {
		t.Fatalf("second import: inserted=%d updated=%d, want inserted=0 updated=2", res.Inserted, res.Updated)
	}

	records, err := store.ListKKT()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected re-import to update rather than duplicate, got %d records", len(records))
	}
}

func TestImportMissingRequiredColumn(t *testing.T) {
	store := newTestDB(t)
	_, err := Import(store, strings.NewReader(`="Модель ККТ";="Адрес расчетов"`+"\r\n"+`="X";="Y"`), "ИП Пупкин И.В.")
	if err == nil {
		t.Fatal("expected an error for a CSV missing required columns")
	}
}

func TestNormalizeAddress(t *testing.T) {
	cases := []struct{ in, want string }{
		{
			"г Пенза, ул Бийская, д. 7",
			"г Пенза, ул Бийская, д. 7",
		},
		{
			"58 - Пензенская область, 440072, г Пенза,                   ул Антонова, стр 18В",
			"г Пенза, ул Антонова, стр 18В",
		},
		{
			"440528, Пензенская обл. с. Богословка, ул. Дорожная, д. 1",
			"с. Богословка, ул. Дорожная, д. 1",
		},
		{
			"58 - Пензенская область, г.о. город Пенза, 440007, гПенза, ул Измайлова, д. 58А, к.",
			"г Пенза, ул Измайлова, д. 58А, к.",
		},
		{
			"м.р-н Пензенский, с Засечное, ул Семейная, д. 12",
			"с Засечное, ул Семейная, д. 12",
		},
		{
			"440046, РОССИЯ, 58, город Пенза г.о., Пенза г.,Мираул.,д. 44А",
			"г Пенза, Мираул.,д. 44А",
		},
		// republic given as "keyword + name", other regions/krais.
		{
			"Республика Татарстан, г. Казань, ул. Баумана, д. 1",
			"г. Казань, ул. Баумана, д. 1",
		},
		{
			"Краснодарский край, г. Сочи, ул. Навагинская, д. 10",
			"г. Сочи, ул. Навагинская, д. 10",
		},
		// double-barrel autonomous okrug name ("округ - Югра").
		{
			"Ханты-Мансийский автономный округ - Югра, г. Сургут, ул. Ленина, д. 5",
			"г. Сургут, ул. Ленина, д. 5",
		},
		{"", ""},
	}
	for _, c := range cases {
		got := normalizeAddress(c.in)
		if got != c.want {
			t.Errorf("normalizeAddress(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
