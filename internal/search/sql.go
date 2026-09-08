package search

import (
	"context"

	"bookreviews/internal/models"
	"bookreviews/internal/store"
)

// SQL es el fallback: LIKE sobre el resumen
type SQL struct {
	Store *store.Store
}

func (SQL) Name() string { return "SQLite" }

func (s SQL) Search(ctx context.Context, terms []string, page store.Page) ([]store.BookWithAuthor, store.Page, error) {
	return s.Store.Books.Search(ctx, terms, page)
}

func (SQL) IndexBook(context.Context, models.Book) error     { return nil }
func (SQL) DeleteBook(context.Context, int64) error          { return nil }
func (SQL) IndexReview(context.Context, models.Review) error { return nil }
func (SQL) DeleteReview(context.Context, int64) error        { return nil }
func (SQL) ReindexAll(context.Context) error                 { return nil }
