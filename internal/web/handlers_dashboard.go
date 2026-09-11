package web

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"kkt-monitor/internal/db"
)

type dateStatus struct {
	Date     string
	DaysLeft int
	HasDays  bool
	Class    string // "", "ok", "warn", "danger"
}

type kktRow struct {
	db.KKT
	OFD dateStatus
	FN  dateStatus
}

func computeStatus(dateStr string, today time.Time) dateStatus {
	if dateStr == "" {
		return dateStatus{}
	}
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return dateStatus{Date: dateStr}
	}
	days := int(t.Sub(today).Hours() / 24)
	class := "ok"
	switch {
	case days < 0:
		class = "danger"
	case days <= 5:
		class = "danger"
	case days <= 30:
		class = "warn"
	}
	return dateStatus{Date: dateStr, DaysLeft: days, HasDays: true, Class: class}
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	records, err := s.store.ListKKT()
	if err != nil {
		http.Error(w, "ошибка чтения базы", http.StatusInternalServerError)
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	needle := strings.ToLower(query)

	today := time.Now().Truncate(24 * time.Hour)
	rows := make([]kktRow, 0, len(records))
	expiringSoon := 0
	total := 0
	for _, k := range records {
		if needle != "" &&
			!strings.Contains(strings.ToLower(k.Organization), needle) &&
			!strings.Contains(strings.ToLower(k.Address), needle) {
			continue
		}
		total++
		row := kktRow{KKT: k, OFD: computeStatus(k.OFDEndDate, today), FN: computeStatus(k.FNEndDate, today)}
		if row.OFD.Class == "danger" || row.OFD.Class == "warn" || row.FN.Class == "danger" || row.FN.Class == "warn" {
			expiringSoon++
		}
		rows = append(rows, row)
	}

	s.render(w, "dashboard.html", map[string]any{
		"Rows":         rows,
		"Total":        total,
		"ExpiringSoon": expiringSoon,
		"Query":        query,
		"IsAdmin":      s.isAuthenticated(r),
	})
}

func (s *Server) handleDeleteKKT(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	if err := s.store.DeleteKKT(id); err != nil {
		http.Error(w, "ошибка удаления", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleEditKKTForm(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	k, err := s.store.GetKKT(id)
	if err != nil {
		http.Error(w, "запись не найдена", http.StatusNotFound)
		return
	}
	s.render(w, "edit_kkt.html", map[string]any{"KKT": k})
}

func (s *Server) handleEditKKTSubmit(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	k := db.KKT{
		ID:           id,
		Organization: strings.TrimSpace(r.FormValue("organization")),
		RegNumber:    strings.TrimSpace(r.FormValue("reg_number")),
		FNNumber:     strings.TrimSpace(r.FormValue("fn_number")),
		SerialNumber: strings.TrimSpace(r.FormValue("serial_number")),
		Address:      strings.TrimSpace(r.FormValue("address")),
		Model:        strings.TrimSpace(r.FormValue("model")),
		OFDEndDate:   strings.TrimSpace(r.FormValue("ofd_end_date")),
		FNEndDate:    strings.TrimSpace(r.FormValue("fn_end_date")),
	}

	if k.SerialNumber == "" {
		s.render(w, "edit_kkt.html", map[string]any{"KKT": k, "Error": "Заводской номер ККТ обязателен"})
		return
	}
	for _, d := range []struct{ label, value string }{{"Дата окончания услуг (ОФД)", k.OFDEndDate}, {"Дата окончания срока ФН", k.FNEndDate}} {
		if d.value == "" {
			continue
		}
		if _, err := time.Parse("2006-01-02", d.value); err != nil {
			s.render(w, "edit_kkt.html", map[string]any{"KKT": k, "Error": d.label + ": неверный формат даты"})
			return
		}
	}

	if err := s.store.UpdateKKT(id, k); err != nil {
		msg := "Ошибка сохранения: " + err.Error()
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			msg = "ККТ с таким заводским номером уже есть в реестре"
		}
		s.render(w, "edit_kkt.html", map[string]any{"KKT": k, "Error": msg})
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
