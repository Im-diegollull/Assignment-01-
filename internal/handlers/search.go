package handlers

import (
	"net/http"
	"net/url"

	"bookreviews/internal/store"
)

// searchPageSize es más chico que el de los listados: los resultados muestran
// un fragmento del resumen y ocupan bastante más alto que una fila de tabla.
const searchPageSize = 10

type searchPage struct {
	Query string

	Terms      []string
	Books      []store.BookWithAuthor
	Pagination pagination
	Backend    string

	Searched bool
}

func (h *Handler) search(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	terms := store.SearchTerms(query)

	page := store.NewPage(pageNumber(r), searchPageSize)
	books, page, err := h.searcher.Search(r.Context(), terms, page)
	if err != nil {
		h.serverError(w, err)
		return
	}

	keep := url.Values{}
	if query != "" {
		keep.Set("q", query)
	}

	h.render(w, http.StatusOK, "search.html", searchPage{
		Query:      query,
		Terms:      terms,
		Books:      books,
		Pagination: newPagination("/search", keep, page),
		Backend:    h.searcher.Name(),
		Searched:   len(terms) > 0,
	})
}
