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

	"bookreviews/internal/search"
	"bookreviews/internal/stats"
	"bookreviews/internal/store"
	"bookreviews/web"
)

// Handler agrupa las dependencias de la capa HTTP. Se construye una sola vez
// en cmd/server/main.go; no hay estado global.
type Handler struct {
	logger    *slog.Logger
	store     *store.Store
	stats     *stats.Reader
	searcher  search.Engine
	templates map[string]*template.Template
	mediaRoot string
	proxyMode bool
}

// New construye el Handler y deja las plantillas parseadas de antemano.
// reader o engine nil caen a cache noop y búsqueda SQL.
func New(logger *slog.Logger, st *store.Store, reader *stats.Reader, engine search.Engine) (*Handler, error) {
	if reader == nil {
		reader = stats.New(st, nil, logger)
	}
	if engine == nil {
		engine = search.SQL{Store: st}
	}
	templates, err := parseTemplatesWithDebug(reader.CacheName(), engine.Name())
	if err != nil {
		return nil, err
	}
	return &Handler{logger: logger, store: st, stats: reader, searcher: engine, templates: templates, mediaRoot: "data/media"}, nil
}

func (h *Handler) ConfigureFiles(mediaRoot string, proxyMode bool) {
	h.mediaRoot = mediaRoot
	h.proxyMode = proxyMode
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()

	// FileServerFS sirve directamente desde el embed.FS: la ruta /static/x.css
	// se resuelve como "static/x.css" dentro del FS, sin StripPrefix.
	if !h.proxyMode {
		mux.Handle("GET /static/", http.FileServerFS(web.Files))
		mux.Handle("GET /media/", http.StripPrefix("/media/", http.FileServer(http.Dir(h.mediaRoot))))
	}

	mux.HandleFunc("GET /{$}", h.home)
	mux.HandleFunc("GET /healthz", h.healthz)
	mux.HandleFunc("GET /recent", func(w http.ResponseWriter, r *http.Request) {
		h.redirect(w, r, "/books")
	})

	mux.HandleFunc("GET /search", h.search)
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
	mux.HandleFunc("POST /authors/{id}/image", h.uploadAuthorImage)

	mux.HandleFunc("GET /books", h.bookList)
	mux.HandleFunc("GET /books/new", h.bookNew)
	mux.HandleFunc("POST /books", h.bookCreate)
	mux.HandleFunc("GET /books/{id}", h.bookShow)
	mux.HandleFunc("GET /books/{id}/edit", h.bookEdit)
	mux.HandleFunc("POST /books/{id}/edit", h.bookUpdate)
	mux.HandleFunc("POST /books/{id}/delete", h.bookDelete)
	mux.HandleFunc("POST /books/{id}/image", h.uploadBookImage)

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
	w.Header().Set("X-Search", h.searcher.Name())
	w.Header().Set("X-Cache-Backend", h.stats.CacheName())
	w.WriteHeader(status)
	if _, err := buf.WriteTo(w); err != nil {
		h.logger.Error("no se pudo escribir la respuesta", "page", page, "error", err)
	}
}

func (h *Handler) redirect(w http.ResponseWriter, r *http.Request, url string) {
	http.Redirect(w, r, url, http.StatusSeeOther)
}

func (h *Handler) serverError(w http.ResponseWriter, err error) {
	h.logger.Error("error interno", "error", err)
	http.Error(w, "Error interno del servidor", http.StatusInternalServerError)
}

func (h *Handler) notFound(w http.ResponseWriter) {
	http.Error(w, "No encontrado", http.StatusNotFound)
}

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

func parseInt64(raw string) (int64, bool) {
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return 0, false
	}
	return value, true
}

func pageParam(r *http.Request) store.Page {
	return store.NewPage(pageNumber(r), store.DefaultPageSize)
}

func pageNumber(r *http.Request) int {
	number, _ := strconv.Atoi(r.URL.Query().Get("page"))
	return number
}
