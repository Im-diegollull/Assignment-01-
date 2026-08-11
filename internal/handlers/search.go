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
	// Query es lo que el usuario escribió, tal cual, para dejarlo en el input.
	Query string
	// Terms son las palabras que efectivamente se buscaron. Se muestran porque
	// no siempre coinciden con lo escrito: se descartan las de un carácter, las
	// repetidas y todo lo que pase de MaxSearchTerms.
	Terms      []string
	Books      []store.BookWithAuthor
	Pagination pagination
	// Searched distingue "todavía no buscaste nada" de "buscaste y no hubo
	// resultados", que son dos estados vacíos con mensajes distintos.
	Searched bool
}

func (h *Handler) search(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	terms := store.SearchTerms(query)

	page := store.NewPage(pageNumber(r), searchPageSize)
	books, page, err := h.store.Books.Search(r.Context(), terms, page)
	if err != nil {
		h.serverError(w, err)
		return
	}

	// Solo se conserva el término en los enlaces de paginación; el ?page= lo
	// pone newPagination.
	keep := url.Values{}
	if query != "" {
		keep.Set("q", query)
	}

	h.render(w, http.StatusOK, "search.html", searchPage{
		Query:      query,
		Terms:      terms,
		Books:      books,
		Pagination: newPagination("/search", keep, page),
		Searched:   len(terms) > 0,
	})
}
