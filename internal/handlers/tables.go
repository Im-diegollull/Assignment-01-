package handlers

import (
	"net/http"
	"net/url"
	"strconv"

	"bookreviews/internal/store"
)

const (
	topRatedLimit   = 10
	topSellingLimit = 50
)

// columnHeader es el encabezado clicable de una columna ordenable. El handler
// arma la URL completa para que la plantilla no tenga que saber nada de query
// params ni de cómo se alterna la dirección.
type columnHeader struct {
	Label     string
	URL       string
	Active    bool
	Ascending bool
	Numeric   bool
}

type authorStatsPage struct {
	Rows    []store.AuthorStatsRow
	Headers []columnHeader
	Filters authorStatsFilters

	Ignored []string
}

type authorStatsFilters struct {
	Name     string
	MinBooks string
	MaxBooks string
	MinScore string
	MaxScore string
	MinSales string
	MaxSales string
}

func (h *Handler) authorStats(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	filters := authorStatsFilters{
		Name:     query.Get("name"),
		MinBooks: query.Get("min_books"),
		MaxBooks: query.Get("max_books"),
		MinScore: query.Get("min_score"),
		MaxScore: query.Get("max_score"),
		MinSales: query.Get("min_sales"),
		MaxSales: query.Get("max_sales"),
	}

	filter, ignored := filters.toStoreFilter()
	filter.SortBy = query.Get("sort")
	filter.Dir = query.Get("dir")

	rows, err := h.store.Authors.AuthorStats(r.Context(), filter)
	if err != nil {
		h.serverError(w, err)
		return
	}

	h.render(w, http.StatusOK, "authors_stats.html", authorStatsPage{
		Rows:    rows,
		Headers: authorStatsHeaders(query),
		Filters: filters,
		Ignored: ignored,
	})
}

func (f authorStatsFilters) toStoreFilter() (store.AuthorFilter, []string) {
	var ignored []string

	intField := func(raw, label string) *int {
		value, ok := parseOptionalInt(raw)
		if !ok {
			ignored = append(ignored, label)
		}
		return value
	}
	floatField := func(raw, label string) *float64 {
		value, ok := parseOptionalFloat(raw)
		if !ok {
			ignored = append(ignored, label)
		}
		return value
	}

	minBooks := intField(f.MinBooks, "libros (mínimo)")
	maxBooks := intField(f.MaxBooks, "libros (máximo)")
	minScore := floatField(f.MinScore, "score (mínimo)")
	maxScore := floatField(f.MaxScore, "score (máximo)")
	minSales := intField(f.MinSales, "ventas (mínimo)")
	maxSales := intField(f.MaxSales, "ventas (máximo)")

	filter := store.AuthorFilter{
		NameLike: f.Name,
		MinBooks: minBooks,
		MaxBooks: maxBooks,
		MinScore: minScore,
		MaxScore: maxScore,
		MinSales: minSales,
		MaxSales: maxSales,
	}
	return filter, ignored
}

func authorStatsHeaders(query url.Values) []columnHeader {
	columns := []struct {
		key     string
		label   string
		numeric bool
	}{
		{"name", "Autor", false},
		{"books", "Libros publicados", true},
		{"avg_score", "Score promedio", true},
		{"total_sales", "Ventas totales", true},
	}

	activeSort := query.Get("sort")
	if activeSort == "" {
		activeSort = "name"
	}
	activeAsc := query.Get("dir") != "desc"

	headers := make([]columnHeader, 0, len(columns))
	for _, column := range columns {
		isActive := column.key == activeSort

		nextDir := "asc"
		if isActive && activeAsc {
			nextDir = "desc"
		}

		next := cloneQuery(query)
		next.Set("sort", column.key)
		next.Set("dir", nextDir)

		headers = append(headers, columnHeader{
			Label:     column.label,
			URL:       "/authors/stats?" + next.Encode(),
			Active:    isActive,
			Ascending: isActive && activeAsc,
			Numeric:   column.numeric,
		})
	}
	return headers
}

func (h *Handler) topRatedBooks(w http.ResponseWriter, r *http.Request) {
	rows, err := h.store.Books.TopRated(r.Context(), topRatedLimit)
	if err != nil {
		h.serverError(w, err)
		return
	}
	h.render(w, http.StatusOK, "books_top_rated.html", rows)
}

func (h *Handler) topSellingBooks(w http.ResponseWriter, r *http.Request) {
	rows, err := h.store.Books.TopSelling(r.Context(), topSellingLimit)
	if err != nil {
		h.serverError(w, err)
		return
	}
	h.render(w, http.StatusOK, "books_top_selling.html", rows)
}

func parseOptionalInt(raw string) (*int, bool) {
	if raw == "" {
		return nil, true
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return nil, false
	}
	return &value, true
}

func parseOptionalFloat(raw string) (*float64, bool) {
	if raw == "" {
		return nil, true
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return nil, false
	}
	return &value, true
}

func cloneQuery(query url.Values) url.Values {
	clone := make(url.Values, len(query))
	for key, values := range query {
		clone[key] = append([]string(nil), values...)
	}
	return clone
}
