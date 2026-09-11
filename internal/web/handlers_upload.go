package web

import (
	"fmt"
	"net/http"

	"kkt-monitor/internal/csvimport"
)

func (s *Server) handleUploadForm(w http.ResponseWriter, r *http.Request) {
	s.render(w, "upload.html", map[string]any{})
}

func (s *Server) handleUploadSubmit(w http.ResponseWriter, r *http.Request) {
	// 32 MiB in-memory limit for parsed multipart parts; the file itself
	// spills to a temp file beyond that if larger.
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		s.render(w, "upload.html", map[string]any{"Error": "Не удалось прочитать форму: " + err.Error()})
		return
	}
	organization := r.FormValue("organization")
	if organization == "" {
		s.render(w, "upload.html", map[string]any{"Error": "Укажите название организации, которой принадлежит файл"})
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		s.render(w, "upload.html", map[string]any{"Error": "Выберите CSV-файл для загрузки", "Organization": organization})
		return
	}
	defer file.Close()

	result, err := csvimport.Import(s.store, file, organization)
	if err != nil {
		s.render(w, "upload.html", map[string]any{"Error": "Ошибка импорта: " + err.Error()})
		return
	}

	s.render(w, "upload.html", map[string]any{
		"Success": fmt.Sprintf("Импорт завершён: добавлено %d, обновлено %d, пропущено %d.", result.Inserted, result.Updated, result.Skipped),
		"Errors":  result.Errors,
	})
}
