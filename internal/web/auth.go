package web

import (
	"context"
	"crypto/subtle"
	"net/http"
	"time"

	"kkt-monitor/internal/db"
)

const sessionCookieName = "kkt_session"

// checkCredentials matches password against the configured admin/viewer
// passwords and returns which role it grants, if any.
func (s *Server) checkCredentials(password string) (role string, ok bool) {
	if password == "" {
		return "", false
	}
	if s.cfg.AdminPassword != "" && subtle.ConstantTimeCompare([]byte(password), []byte(s.cfg.AdminPassword)) == 1 {
		return db.RoleAdmin, true
	}
	if s.cfg.ViewerPassword != "" && subtle.ConstantTimeCompare([]byte(password), []byte(s.cfg.ViewerPassword)) == 1 {
		return db.RoleViewer, true
	}
	return "", false
}

func (s *Server) startSession(w http.ResponseWriter, r *http.Request, role string) error {
	token, err := s.store.CreateSession(role)
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

// sessionRole returns the role of the current request's session, if any.
func (s *Server) sessionRole(r *http.Request) (role string, ok bool) {
	c, err := r.Cookie(sessionCookieName)
	if err != nil {
		return "", false
	}
	return s.store.SessionRole(c.Value)
}

type ctxKey int

const roleKey ctxKey = 0

func roleFromContext(r *http.Request) string {
	role, _ := r.Context().Value(roleKey).(string)
	return role
}

// requireAuth redirects to /login when there is no valid session of any role.
func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		role, ok := s.sessionRole(r)
		if !ok {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		ctx := context.WithValue(r.Context(), roleKey, role)
		next(w, r.WithContext(ctx))
	}
}

// requireAdmin redirects to /login when there is no session, and responds
// with 403 when the session is valid but not an admin (e.g. viewer trying
// to reach upload/settings/delete directly by URL).
func (s *Server) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		role, ok := s.sessionRole(r)
		if !ok {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		if role != db.RoleAdmin {
			http.Error(w, "доступ только для администратора", http.StatusForbidden)
			return
		}
		ctx := context.WithValue(r.Context(), roleKey, role)
		next(w, r.WithContext(ctx))
	}
}
