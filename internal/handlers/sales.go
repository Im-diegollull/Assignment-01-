package handlers

import (
	"errors"
	"net/http"
	"time"

	"bookreviews/internal/models"
	"bookreviews/internal/store"
)

type saleListPage struct {
	Sales   []store.SaleWithBook
	Page    store.Page
	BaseURL string
}

type saleShowPage struct {
	Sale store.SaleWithBook
}

type saleFormPage struct {
	Sale   models.Sale
	Books  []models.Book
	Errors models.Errors
	IsEdit bool
}

func (h *Handler) saleList(w http.ResponseWriter, r *http.Request) {
	sales, page, err := h.store.Sales.List(r.Context(), pageParam(r))
	if err != nil {
		h.serverError(w, err)
		return
	}
	h.render(w, http.StatusOK, "sales_list.html", saleListPage{
		Sales: sales, Page: page, BaseURL: "/sales",
	})
}

func (h *Handler) saleShow(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		h.notFound(w)
		return
	}

	sale, err := h.store.Sales.Get(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		h.notFound(w)
		return
	}
	if err != nil {
		h.serverError(w, err)
		return
	}
	h.render(w, http.StatusOK, "sales_show.html", saleShowPage{Sale: sale})
}

func (h *Handler) saleNew(w http.ResponseWriter, r *http.Request) {
	preselected, _ := parseInt64(r.URL.Query().Get("book_id"))
	sale := models.Sale{BookID: preselected, Year: time.Now().Year()}
	h.renderSaleForm(w, r, sale, models.Errors{}, false)
}

func (h *Handler) saleCreate(w http.ResponseWriter, r *http.Request) {
	sale, errs, err := h.parseSaleForm(r)
	if err != nil {
		h.serverError(w, err)
		return
	}
	if len(errs) > 0 {
		h.renderSaleForm(w, r, sale, errs, false)
		return
	}

	err = h.store.Sales.Create(r.Context(), &sale)
	if errors.Is(err, store.ErrDuplicateSaleYear) {
		errs["Year"] = "Ese libro ya tiene ventas registradas para ese año."
		h.renderSaleForm(w, r, sale, errs, false)
		return
	}
	if err != nil {
		h.serverError(w, err)
		return
	}
	h.redirect(w, r, "/sales/"+itoa(sale.ID))
}

func (h *Handler) saleEdit(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		h.notFound(w)
		return
	}

	sale, err := h.store.Sales.Get(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		h.notFound(w)
		return
	}
	if err != nil {
		h.serverError(w, err)
		return
	}
	h.renderSaleForm(w, r, sale.Sale, models.Errors{}, true)
}

func (h *Handler) saleUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		h.notFound(w)
		return
	}

	sale, errs, err := h.parseSaleForm(r)
	if err != nil {
		h.serverError(w, err)
		return
	}
	sale.ID = id

	if len(errs) > 0 {
		h.renderSaleForm(w, r, sale, errs, true)
		return
	}

	err = h.store.Sales.Update(r.Context(), &sale)
	switch {
	case errors.Is(err, store.ErrNotFound):
		h.notFound(w)
	case errors.Is(err, store.ErrDuplicateSaleYear):
		errs["Year"] = "Ese libro ya tiene ventas registradas para ese año."
		h.renderSaleForm(w, r, sale, errs, true)
	case err != nil:
		h.serverError(w, err)
	default:
		h.redirect(w, r, "/sales/"+itoa(id))
	}
}

func (h *Handler) saleDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		h.notFound(w)
		return
	}

	err := h.store.Sales.Delete(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		h.notFound(w)
		return
	}
	if err != nil {
		h.serverError(w, err)
		return
	}
	h.redirect(w, r, "/sales")
}

func (h *Handler) renderSaleForm(w http.ResponseWriter, r *http.Request,
	sale models.Sale, errs models.Errors, isEdit bool) {

	books, err := h.store.Books.All(r.Context())
	if err != nil {
		h.serverError(w, err)
		return
	}

	status := http.StatusOK
	if len(errs) > 0 {
		status = http.StatusUnprocessableEntity
	}
	h.render(w, status, "sales_form.html", saleFormPage{
		Sale: sale, Books: books, Errors: errs, IsEdit: isEdit,
	})
}

func (h *Handler) parseSaleForm(r *http.Request) (models.Sale, models.Errors, error) {
	f, err := newForm(r)
	if err != nil {
		return models.Sale{}, nil, err
	}

	sale := models.Sale{
		BookID: f.id("BookID", "El libro"),
		Year:   f.integer("Year", "El año"),
		Sales:  f.integer("Sales", "Las ventas"),
	}
	return sale, f.merge(sale.Validate()), nil
}
