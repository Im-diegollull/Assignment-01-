package handlers

import (
	"net/url"

	"bookreviews/internal/store"
)

// pagination es lo que consume el parcial "pagination". Los enlaces vienen ya
// armados desde el handler porque la página anterior y la siguiente tienen que
// conservar el resto de los parámetros —el ?q= de la búsqueda, por ejemplo— y
// concatenar "?page=N" a mano se rompe en cuanto la URL ya trae query string.
type pagination struct {
	Page    store.Page
	PrevURL string
	NextURL string
}

// newPagination arma los enlaces de navegación. keep son los parámetros que hay
// que conservar; se copia antes de tocarlos para no modificar los del request.
func newPagination(path string, keep url.Values, page store.Page) pagination {
	links := pagination{Page: page}

	pageURL := func(number int) string {
		values := cloneQuery(keep)
		values.Set("page", itoa(int64(number)))
		return path + "?" + values.Encode()
	}

	if page.HasPrev() {
		links.PrevURL = pageURL(page.PrevNumber())
	}
	if page.HasNext() {
		links.NextURL = pageURL(page.NextNumber())
	}
	return links
}
