
package search

import (
	"context"

	"bookreviews/internal/models"
	"bookreviews/internal/store"
)

type Engine interface {
	Name() string
	Search(ctx context.Context, terms []string, page store.Page) ([]store.BookWithAuthor, store.Page, error)
	IndexBook(ctx context.Context, book models.Book) error
	DeleteBook(ctx context.Context, bookID int64) error
	IndexReview(ctx context.Context, review models.Review) error
	DeleteReview(ctx context.Context, reviewID int64) error
	ReindexAll(ctx context.Context) error
}
