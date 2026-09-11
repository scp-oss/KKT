package web

import (
	"net/http"
	"time"
)

func (s *Server) handleLoginForm(w http.ResponseWriter, r *http.Request) {
	if s.isAuthenticated(r) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	s.render(w, "login.html", map[string]any{
		"Error": r.URL.Query().Get("error"),
	})
}

func (s *Server) handleLoginSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	password := r.FormValue("password")

	if !s.checkPassword(password) {
		// small delay to blunt brute-force attempts
		time.Sleep(500 * time.Millisecond)
		http.Redirect(w, r, "/login?error=1", http.StatusSeeOther)
		return
	}

	if err := s.startSession(w, r); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	s.endSession(w, r)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
