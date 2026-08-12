package handlers

import (
	"net/url"

	"bookreviews/internal/store"
)

type pagination struct {
	Page    store.Page
	PrevURL string
	NextURL string
}

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
