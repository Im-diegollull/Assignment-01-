// Package handlers contiene la capa HTTP: parseo de formularios, validación de
// entrada y render de plantillas. No contiene SQL; toda consulta vive en
// internal/store.
package handlers

import (
	"bytes"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"

	"bookreviews/web"
)

// Handler agrupa las dependencias de la capa HTTP. Se construye una sola vez
// en cmd/server/main.go; no hay estado global.
type Handler struct {
	logger    *slog.Logger
	templates map[string]*template.Template
}

// New construye el Handler y deja las plantillas parseadas de antemano, para
// que un error de sintaxis en un template falle al arrancar y no en runtime.
func New(logger *slog.Logger) (*Handler, error) {
	templates, err := parseTemplates()
	if err != nil {
		return nil, err
	}
	return &Handler{logger: logger, templates: templates}, nil
}

// Routes declara el ruteo con el http.ServeMux de la stdlib (Go 1.22+), que ya
// soporta método y wildcards en el patrón.
func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()

	// FileServerFS sirve directamente desde el embed.FS: la ruta /static/x.css
	// se resuelve como "static/x.css" dentro del FS, sin StripPrefix.
	mux.Handle("GET /static/", http.FileServerFS(web.Files))

	// "/{$}" matchea exactamente la raíz; sin el {$} sería un catch-all.
	mux.HandleFunc("GET /{$}", h.home)

	return mux
}

// render escribe una página completa. La plantilla se ejecuta primero contra un
// buffer: si falla a mitad de camino, el cliente recibe un 500 limpio en vez de
// un HTML truncado con status 200 ya enviado.
func (h *Handler) render(w http.ResponseWriter, status int, page string, data any) {
	ts, ok := h.templates[page]
	if !ok {
		h.serverError(w, fmt.Errorf("handlers: plantilla %q no encontrada", page))
		return
	}

	var buf bytes.Buffer
	if err := ts.ExecuteTemplate(&buf, "base", data); err != nil {
		h.serverError(w, fmt.Errorf("handlers: ejecutar %q: %w", page, err))
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if _, err := buf.WriteTo(w); err != nil {
		h.logger.Error("no se pudo escribir la respuesta", "page", page, "error", err)
	}
}

// serverError registra el error real y devuelve un 500 genérico al cliente.
func (h *Handler) serverError(w http.ResponseWriter, err error) {
	h.logger.Error("error interno", "error", err)
	http.Error(w, "Error interno del servidor", http.StatusInternalServerError)
}
