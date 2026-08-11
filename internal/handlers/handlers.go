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
	"strconv"

	"bookreviews/internal/store"
	"bookreviews/web"
)

// Handler agrupa las dependencias de la capa HTTP. Se construye una sola vez
// en cmd/server/main.go; no hay estado global.
type Handler struct {
	logger    *slog.Logger
	store     *store.Store
	templates map[string]*template.Template
}

// New construye el Handler y deja las plantillas parseadas de antemano, para
// que un error de sintaxis en un template falle al arrancar y no en runtime.
func New(logger *slog.Logger, st *store.Store) (*Handler, error) {
	templates, err := parseTemplates()
	if err != nil {
		return nil, err
	}
	return &Handler{logger: logger, store: st, templates: templates}, nil
}

// Routes declara el ruteo con el http.ServeMux de la stdlib (Go 1.22+), que ya
// soporta método y wildcards en el patrón.
//
// Los formularios HTML solo saben hacer GET y POST, así que las acciones
// destructivas tienen su propia ruta POST explícita en vez de un método
// simulado con un campo _method.
func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()

	// FileServerFS sirve directamente desde el embed.FS: la ruta /static/x.css
	// se resuelve como "static/x.css" dentro del FS, sin StripPrefix.
	mux.Handle("GET /static/", http.FileServerFS(web.Files))

	// "/{$}" matchea exactamente la raíz; sin el {$} sería un catch-all.
	mux.HandleFunc("GET /{$}", h.home)

	// El patrón literal /authors/new gana sobre el wildcard /authors/{id}:
	// ServeMux elige siempre el más específico, sin importar el orden.
	// Tablas de reporte. Van antes conceptualmente, pero el orden de registro
	// no importa: ServeMux elige por especificidad, no por orden.
	mux.HandleFunc("GET /authors/stats", h.authorStats)
	mux.HandleFunc("GET /books/top-rated", h.topRatedBooks)
	mux.HandleFunc("GET /books/top-selling", h.topSellingBooks)

	mux.HandleFunc("GET /authors", h.authorList)
	mux.HandleFunc("GET /authors/new", h.authorNew)
	mux.HandleFunc("POST /authors", h.authorCreate)
	mux.HandleFunc("GET /authors/{id}", h.authorShow)
	mux.HandleFunc("GET /authors/{id}/edit", h.authorEdit)
	mux.HandleFunc("POST /authors/{id}/edit", h.authorUpdate)
	mux.HandleFunc("POST /authors/{id}/delete", h.authorDelete)

	mux.HandleFunc("GET /books", h.bookList)
	mux.HandleFunc("GET /books/new", h.bookNew)
	mux.HandleFunc("POST /books", h.bookCreate)
	mux.HandleFunc("GET /books/{id}", h.bookShow)
	mux.HandleFunc("GET /books/{id}/edit", h.bookEdit)
	mux.HandleFunc("POST /books/{id}/edit", h.bookUpdate)
	mux.HandleFunc("POST /books/{id}/delete", h.bookDelete)

	mux.HandleFunc("GET /reviews", h.reviewList)
	mux.HandleFunc("GET /reviews/new", h.reviewNew)
	mux.HandleFunc("POST /reviews", h.reviewCreate)
	mux.HandleFunc("GET /reviews/{id}", h.reviewShow)
	mux.HandleFunc("GET /reviews/{id}/edit", h.reviewEdit)
	mux.HandleFunc("POST /reviews/{id}/edit", h.reviewUpdate)
	mux.HandleFunc("POST /reviews/{id}/delete", h.reviewDelete)

	mux.HandleFunc("GET /sales", h.saleList)
	mux.HandleFunc("GET /sales/new", h.saleNew)
	mux.HandleFunc("POST /sales", h.saleCreate)
	mux.HandleFunc("GET /sales/{id}", h.saleShow)
	mux.HandleFunc("GET /sales/{id}/edit", h.saleEdit)
	mux.HandleFunc("POST /sales/{id}/edit", h.saleUpdate)
	mux.HandleFunc("POST /sales/{id}/delete", h.saleDelete)

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

// redirect responde a un POST exitoso con 303 See Other, para que recargar la
// página siguiente no reenvíe el formulario.
func (h *Handler) redirect(w http.ResponseWriter, r *http.Request, url string) {
	http.Redirect(w, r, url, http.StatusSeeOther)
}

// serverError registra el error real y devuelve un 500 genérico al cliente.
func (h *Handler) serverError(w http.ResponseWriter, err error) {
	h.logger.Error("error interno", "error", err)
	http.Error(w, "Error interno del servidor", http.StatusInternalServerError)
}

func (h *Handler) notFound(w http.ResponseWriter) {
	http.Error(w, "No encontrado", http.StatusNotFound)
}

// pathID lee el {id} de la ruta. Un id no numérico es un 404 y no un 400: la
// URL simplemente no corresponde a ningún recurso.
func pathID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

func itoa(id int64) string {
	return strconv.FormatInt(id, 10)
}

// parseInt64 se usa para los ?book_id=N que preseleccionan un desplegable. Un
// valor inválido devuelve 0 y el formulario aparece sin preselección.
func parseInt64(raw string) (int64, bool) {
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return 0, false
	}
	return value, true
}

// pageParam lee ?page=N. Un valor ausente o inválido cae en la página 1.
func pageParam(r *http.Request) store.Page {
	number, _ := strconv.Atoi(r.URL.Query().Get("page"))
	return store.NewPage(number, store.DefaultPageSize)
}
