package handlers

import (
	"errors"
	"net/http"

	"bookreviews/internal/models"
	"bookreviews/internal/store"
)

type authorListPage struct {
	Authors []models.Author
	Page    store.Page
	BaseURL string
}

type authorShowPage struct {
	Author models.Author
	Books  []store.BookWithAuthor
}

type authorFormPage struct {
	Author models.Author
	Errors models.Errors
	IsEdit bool
}

func (h *Handler) authorList(w http.ResponseWriter, r *http.Request) {
	authors, page, err := h.store.Authors.List(r.Context(), pageParam(r))
	if err != nil {
		h.serverError(w, err)
		return
	}
	h.render(w, http.StatusOK, "authors_list.html", authorListPage{
		Authors: authors, Page: page, BaseURL: "/authors",
	})
}

func (h *Handler) authorShow(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		h.notFound(w)
		return
	}

	author, err := h.store.Authors.Get(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		h.notFound(w)
		return
	}
	if err != nil {
		h.serverError(w, err)
		return
	}

	books, err := h.store.Books.ListByAuthor(r.Context(), id)
	if err != nil {
		h.serverError(w, err)
		return
	}
	h.render(w, http.StatusOK, "authors_show.html", authorShowPage{Author: author, Books: books})
}

func (h *Handler) authorNew(w http.ResponseWriter, r *http.Request) {
	h.render(w, http.StatusOK, "authors_form.html", authorFormPage{Errors: models.Errors{}})
}

func (h *Handler) authorCreate(w http.ResponseWriter, r *http.Request) {
	author, errs, err := h.parseAuthorForm(r)
	if err != nil {
		h.serverError(w, err)
		return
	}
	if len(errs) > 0 {
		// Se re-renderiza el formulario con lo que el usuario ya había escrito.
		h.render(w, http.StatusUnprocessableEntity, "authors_form.html",
			authorFormPage{Author: author, Errors: errs})
		return
	}

	if err := h.store.Authors.Create(r.Context(), &author); err != nil {
		h.serverError(w, err)
		return
	}
	h.redirect(w, r, authorURL(author.ID))
}

func (h *Handler) authorEdit(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		h.notFound(w)
		return
	}

	author, err := h.store.Authors.Get(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		h.notFound(w)
		return
	}
	if err != nil {
		h.serverError(w, err)
		return
	}
	h.render(w, http.StatusOK, "authors_form.html",
		authorFormPage{Author: author, Errors: models.Errors{}, IsEdit: true})
}

func (h *Handler) authorUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		h.notFound(w)
		return
	}

	author, errs, err := h.parseAuthorForm(r)
	if err != nil {
		h.serverError(w, err)
		return
	}
	author.ID = id

	if len(errs) > 0 {
		h.render(w, http.StatusUnprocessableEntity, "authors_form.html",
			authorFormPage{Author: author, Errors: errs, IsEdit: true})
		return
	}

	err = h.store.Authors.Update(r.Context(), &author)
	if errors.Is(err, store.ErrNotFound) {
		h.notFound(w)
		return
	}
	if err != nil {
		h.serverError(w, err)
		return
	}
	h.redirect(w, r, authorURL(id))
}

func (h *Handler) authorDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		h.notFound(w)
		return
	}

	err := h.store.Authors.Delete(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		h.notFound(w)
		return
	}
	if err != nil {
		h.serverError(w, err)
		return
	}
	h.redirect(w, r, "/authors")
}

// parseAuthorForm arma el autor desde el POST y devuelve los errores de parseo
// combinados con los de validación del dominio.
func (h *Handler) parseAuthorForm(r *http.Request) (models.Author, models.Errors, error) {
	f, err := newForm(r)
	if err != nil {
		return models.Author{}, nil, err
	}

	author := models.Author{
		Name:            f.text("Name"),
		DateOfBirth:     f.text("DateOfBirth"),
		CountryOfOrigin: f.text("CountryOfOrigin"),
		Description:     f.text("Description"),
	}
	return author, f.merge(author.Validate()), nil
}

func authorURL(id int64) string {
	return "/authors/" + itoa(id)
}
