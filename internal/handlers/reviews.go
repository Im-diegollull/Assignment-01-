package handlers

import (
	"errors"
	"net/http"

	"bookreviews/internal/models"
	"bookreviews/internal/store"
)

type reviewListPage struct {
	Reviews    []store.ReviewWithBook
	Pagination pagination
}

type reviewShowPage struct {
	Review store.ReviewWithBook
}

type reviewFormPage struct {
	Review models.Review
	Books  []models.Book
	Errors models.Errors
	IsEdit bool
}

func (h *Handler) reviewList(w http.ResponseWriter, r *http.Request) {
	reviews, page, err := h.store.Reviews.List(r.Context(), pageParam(r))
	if err != nil {
		h.serverError(w, err)
		return
	}
	h.render(w, http.StatusOK, "reviews_list.html", reviewListPage{
		Reviews: reviews, Pagination: newPagination("/reviews", r.URL.Query(), page),
	})
}

func (h *Handler) reviewShow(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		h.notFound(w)
		return
	}

	review, err := h.store.Reviews.Get(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		h.notFound(w)
		return
	}
	if err != nil {
		h.serverError(w, err)
		return
	}
	h.render(w, http.StatusOK, "reviews_show.html", reviewShowPage{Review: review})
}

func (h *Handler) reviewNew(w http.ResponseWriter, r *http.Request) {
	// ?book_id=N permite llegar acá desde la ficha de un libro con el
	// desplegable ya posicionado en ese libro.
	preselected, _ := parseInt64(r.URL.Query().Get("book_id"))
	h.renderReviewForm(w, r, models.Review{BookID: preselected}, models.Errors{}, false)
}

func (h *Handler) reviewCreate(w http.ResponseWriter, r *http.Request) {
	review, errs, err := h.parseReviewForm(r)
	if err != nil {
		h.serverError(w, err)
		return
	}
	if len(errs) > 0 {
		h.renderReviewForm(w, r, review, errs, false)
		return
	}

	if err := h.store.Reviews.Create(r.Context(), &review); err != nil {
		h.serverError(w, err)
		return
	}
	h.afterReviewUpsert(r.Context(), review)
	h.redirect(w, r, "/reviews/"+itoa(review.ID))
}

func (h *Handler) reviewEdit(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		h.notFound(w)
		return
	}

	review, err := h.store.Reviews.Get(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		h.notFound(w)
		return
	}
	if err != nil {
		h.serverError(w, err)
		return
	}
	h.renderReviewForm(w, r, review.Review, models.Errors{}, true)
}

func (h *Handler) reviewUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		h.notFound(w)
		return
	}

	review, errs, err := h.parseReviewForm(r)
	if err != nil {
		h.serverError(w, err)
		return
	}
	review.ID = id

	if len(errs) > 0 {
		h.renderReviewForm(w, r, review, errs, true)
		return
	}

	previous, err := h.store.Reviews.Get(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		h.notFound(w)
		return
	}
	if err != nil {
		h.serverError(w, err)
		return
	}

	err = h.store.Reviews.Update(r.Context(), &review)
	if errors.Is(err, store.ErrNotFound) {
		h.notFound(w)
		return
	}
	if err != nil {
		h.serverError(w, err)
		return
	}
	h.afterReviewUpsert(r.Context(), review, previous.BookID)
	h.redirect(w, r, "/reviews/"+itoa(id))
}

func (h *Handler) reviewDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		h.notFound(w)
		return
	}

	previous, err := h.store.Reviews.Get(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		h.notFound(w)
		return
	}
	if err != nil {
		h.serverError(w, err)
		return
	}

	err = h.store.Reviews.Delete(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		h.notFound(w)
		return
	}
	if err != nil {
		h.serverError(w, err)
		return
	}
	h.afterReviewRemoved(r.Context(), previous.Review)
	h.redirect(w, r, "/reviews")
}

func (h *Handler) renderReviewForm(w http.ResponseWriter, r *http.Request,
	review models.Review, errs models.Errors, isEdit bool) {

	books, err := h.store.Books.All(r.Context())
	if err != nil {
		h.serverError(w, err)
		return
	}

	status := http.StatusOK
	if len(errs) > 0 {
		status = http.StatusUnprocessableEntity
	}
	h.render(w, status, "reviews_form.html", reviewFormPage{
		Review: review, Books: books, Errors: errs, IsEdit: isEdit,
	})
}

func (h *Handler) parseReviewForm(r *http.Request) (models.Review, models.Errors, error) {
	f, err := newForm(r)
	if err != nil {
		return models.Review{}, nil, err
	}

	review := models.Review{
		BookID:  f.id("BookID", "El libro"),
		Text:    f.text("Text"),
		Score:   f.integer("Score", "El puntaje"),
		Upvotes: f.integer("Upvotes", "Los up-votes"),
	}
	return review, f.merge(review.Validate()), nil
}
