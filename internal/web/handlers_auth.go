package web

import (
	"log"
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
	if s.cfg.CookieSecure && r.TLS == nil && r.Header.Get("X-Forwarded-Proto") != "https" {
		// The Secure cookie we just set will be silently dropped by the
		// browser on this plain-HTTP connection, so the login will look
		// like it "did nothing" - every next request will bounce back
		// here with no valid session. Surface the likely cause loudly.
		log.Printf("вход: COOKIE_SECURE=true, но запрос пришёл не по HTTPS (клиент %s) — браузер, скорее всего, отбросит cookie сессии и вход не сработает. Установите COOKIE_SECURE=false, если сервис не работает по HTTPS напрямую или через прокси.", r.RemoteAddr)
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	s.endSession(w, r)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
