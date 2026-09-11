// Package web implements the password-protected HTTP UI: dashboard, CSV
// upload, and Telegram bot settings.
package web

import (
	"embed"
	"html/template"
	"log"
	"net/http"

	"kkt-monitor/internal/config"
	"kkt-monitor/internal/db"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static/*
var staticFS embed.FS

type Server struct {
	cfg   config.Config
	store *db.DB
	tmpl  *template.Template
	mux   *http.ServeMux
}

func New(cfg config.Config, store *db.DB) *Server {
	tmpl := template.Must(template.ParseFS(templateFS, "templates/*.html"))
	s := &Server{cfg: cfg, store: store, tmpl: tmpl, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.Handle("GET /static/", http.FileServerFS(staticFS))

	s.mux.HandleFunc("GET /login", s.handleLoginForm)
	s.mux.HandleFunc("POST /login", s.handleLoginSubmit)
	s.mux.HandleFunc("POST /logout", s.requireAuth(s.handleLogout))

	s.mux.HandleFunc("GET /{$}", s.requireAuth(s.handleDashboard))
	s.mux.HandleFunc("POST /kkt/{id}/delete", s.requireAuth(s.handleDeleteKKT))

	s.mux.HandleFunc("GET /upload", s.requireAuth(s.handleUploadForm))
	s.mux.HandleFunc("POST /upload", s.requireAuth(s.handleUploadSubmit))

	s.mux.HandleFunc("GET /settings", s.requireAuth(s.handleSettingsForm))
	s.mux.HandleFunc("POST /settings/bot", s.requireAuth(s.handleSettingsBotSubmit))
	s.mux.HandleFunc("POST /settings/recipients/add", s.requireAuth(s.handleRecipientAdd))
	s.mux.HandleFunc("POST /settings/recipients/{id}/delete", s.requireAuth(s.handleRecipientDelete))
	s.mux.HandleFunc("POST /settings/recipients/{id}/toggle", s.requireAuth(s.handleRecipientToggle))
	s.mux.HandleFunc("POST /settings/test", s.requireAuth(s.handleSettingsTest))
}

func (s *Server) render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("template %s: %v", name, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}
