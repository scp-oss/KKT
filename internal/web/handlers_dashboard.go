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
		"IsAdmin":      roleFromContext(r) == db.RoleAdmin,
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
