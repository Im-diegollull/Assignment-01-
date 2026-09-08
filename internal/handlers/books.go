package handlers

import (
	"errors"
	"net/http"

	"bookreviews/internal/models"
	"bookreviews/internal/store"
)

type bookListPage struct {
	Books      []store.BookWithAuthor
	Pagination pagination
}

type bookShowPage struct {
	Book    store.BookWithAuthor
	Reviews []store.ReviewWithBook
	Sales   []store.SaleWithBook
}

type bookFormPage struct {
	Book    models.Book
	Authors []models.Author
	Errors  models.Errors
	IsEdit  bool
}

func (h *Handler) bookList(w http.ResponseWriter, r *http.Request) {
	books, page, err := h.store.Books.List(r.Context(), pageParam(r))
	if err != nil {
		h.serverError(w, err)
		return
	}
	h.render(w, http.StatusOK, "books_list.html", bookListPage{
		Books: books, Pagination: newPagination("/books", r.URL.Query(), page),
	})
}

func (h *Handler) bookShow(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		h.notFound(w)
		return
	}

	book, err := h.store.Books.Get(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		h.notFound(w)
		return
	}
	if err != nil {
		h.serverError(w, err)
		return
	}

	reviews, err := h.store.Reviews.ListByBook(r.Context(), id)
	if err != nil {
		h.serverError(w, err)
		return
	}
	sales, err := h.store.Sales.ListByBook(r.Context(), id)
	if err != nil {
		h.serverError(w, err)
		return
	}
	h.render(w, http.StatusOK, "books_show.html",
		bookShowPage{Book: book, Reviews: reviews, Sales: sales})
}

func (h *Handler) bookNew(w http.ResponseWriter, r *http.Request) {
	authors, err := h.store.Authors.All(r.Context())
	if err != nil {
		h.serverError(w, err)
		return
	}
	h.render(w, http.StatusOK, "books_form.html",
		bookFormPage{Authors: authors, Errors: models.Errors{}})
}

func (h *Handler) bookCreate(w http.ResponseWriter, r *http.Request) {
	book, errs, err := h.parseBookForm(r)
	if err != nil {
		h.serverError(w, err)
		return
	}
	if len(errs) > 0 {
		h.renderBookForm(w, r, book, errs, false)
		return
	}

	if err := h.store.Books.Create(r.Context(), &book); err != nil {
		h.serverError(w, err)
		return
	}
	h.afterBookUpsert(r.Context(), book)
	h.redirect(w, r, bookURL(book.ID))
}

func (h *Handler) bookEdit(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		h.notFound(w)
		return
	}

	book, err := h.store.Books.Get(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		h.notFound(w)
		return
	}
	if err != nil {
		h.serverError(w, err)
		return
	}
	h.renderBookForm(w, r, book.Book, models.Errors{}, true)
}

func (h *Handler) bookUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		h.notFound(w)
		return
	}

	book, errs, err := h.parseBookForm(r)
	if err != nil {
		h.serverError(w, err)
		return
	}
	book.ID = id

	if len(errs) > 0 {
		h.renderBookForm(w, r, book, errs, true)
		return
	}

	err = h.store.Books.Update(r.Context(), &book)
	if errors.Is(err, store.ErrNotFound) {
		h.notFound(w)
		return
	}
	if err != nil {
		h.serverError(w, err)
		return
	}
	h.afterBookUpsert(r.Context(), book)
	h.redirect(w, r, bookURL(id))
}

func (h *Handler) bookDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		h.notFound(w)
		return
	}

	err := h.store.Books.Delete(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		h.notFound(w)
		return
	}
	if err != nil {
		h.serverError(w, err)
		return
	}
	h.afterBookRemoved(r.Context(), id)
	h.redirect(w, r, "/books")
}

// renderBookForm recarga la lista de autores del <select> antes de dibujar el
// formulario. Se hace en un solo lugar porque hay cuatro caminos que llegan acá
// y todos necesitan lo mismo.
func (h *Handler) renderBookForm(w http.ResponseWriter, r *http.Request,
	book models.Book, errs models.Errors, isEdit bool) {

	authors, err := h.store.Authors.All(r.Context())
	if err != nil {
		h.serverError(w, err)
		return
	}

	status := http.StatusOK
	if len(errs) > 0 {
		status = http.StatusUnprocessableEntity
	}
	h.render(w, status, "books_form.html", bookFormPage{
		Book: book, Authors: authors, Errors: errs, IsEdit: isEdit,
	})
}

func (h *Handler) parseBookForm(r *http.Request) (models.Book, models.Errors, error) {
	f, err := newForm(r)
	if err != nil {
		return models.Book{}, nil, err
	}

	book := models.Book{
		AuthorID:        f.id("AuthorID", "El autor"),
		Name:            f.text("Name"),
		Summary:         f.text("Summary"),
		PublicationDate: f.text("PublicationDate"),
		NumberOfSales:   f.integer("NumberOfSales", "El número de ventas"),
	}
	return book, f.merge(book.Validate()), nil
}

func bookURL(id int64) string {
	return "/books/" + itoa(id)
}
