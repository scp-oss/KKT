package web

import (
	"context"
	"crypto/subtle"
	"net/http"
	"time"
)

const sessionCookieName = "kkt_session"

func (s *Server) checkPassword(password string) bool {
	if s.cfg.AdminPassword == "" {
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

func (s *Server) isAuthenticated(r *http.Request) bool {
	c, err := r.Cookie(sessionCookieName)
	if err != nil {
		return false
	}
	return s.store.ValidSession(c.Value)
}

type ctxKey int

const authedKey ctxKey = 0

// requireAuth redirects to /login when there is no valid session.
func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.isAuthenticated(r) {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		ctx := context.WithValue(r.Context(), authedKey, true)
		next(w, r.WithContext(ctx))
	}
}
