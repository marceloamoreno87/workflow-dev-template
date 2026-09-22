// internal/dashboard/views.go
package dashboard

import (
	"embed"
	"html/template"
	"net/http"

	"github.com/marceloamoreno87/workflow-dev-template/internal/workflow"
)

//go:embed templates/*.html
var templateFiles embed.FS

var listTemplate = template.Must(template.ParseFS(templateFiles, "templates/list.html"))

var detailTemplate = template.Must(template.ParseFS(templateFiles, "templates/detail.html"))

// Items exposes the projection store for wiring.
func (s *Server) Items() *Store {
	return s.store
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := listTemplate.Execute(w, s.store.List()); err != nil {
		http.Error(w, "render failed", http.StatusInternalServerError)
	}
}

func (s *Server) handleItem(w http.ResponseWriter, r *http.Request) {
	item, ok := s.store.Get(workflow.WorkItemID(r.PathValue("id")))
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := detailTemplate.Execute(w, item); err != nil {
		http.Error(w, "render failed", http.StatusInternalServerError)
	}
}
