package web

import (
	"crypto/subtle"
	"net/http"
	"time"
)

const sessionCookieName = "kkt_session"

func (s *Server) checkPassword(password string) bool {
	if password == "" || s.cfg.AdminPassword == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(password), []byte(s.cfg.AdminPassword)) == 1
}

func (s *Server) startSession(w http.ResponseWriter, r *http.Request) error {
	token, err := s.store.CreateSession()
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(30 * 24 * time.Hour),
	})
	return nil
}

func (s *Server) endSession(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookieName); err == nil {
		s.store.DeleteSession(c.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// isAuthenticated reports whether the request carries a valid admin session.
// The dashboard itself is public and never calls this to gate access - only
// to decide whether to show admin-only controls (upload/settings/delete).
func (s *Server) isAuthenticated(r *http.Request) bool {
	c, err := r.Cookie(sessionCookieName)
	if err != nil {
		return false
	}
	return s.store.ValidSession(c.Value)
}

// requireAuth redirects to /login when there is no valid session. Used only
// for admin actions (upload, settings, delete) - viewing the dashboard needs
// no auth at all.
func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.isAuthenticated(r) {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}
