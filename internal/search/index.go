package search

import (
	"context"
	"errors"
	"fmt"

	"bookreviews/internal/store"
)

func reindexFromStore(ctx context.Context, eng Engine, st *store.Store) error {
	books, _, err := st.Books.List(ctx, store.NewPage(1, 100_000))
	if err != nil {
		return fmt.Errorf("search: listar libros para reindexar: %w", err)
	}
	for _, book := range books {
		if err := eng.IndexBook(ctx, book.Book); err != nil {
			return err
		}
	}

	reviews, _, err := st.Reviews.List(ctx, store.NewPage(1, 100_000))
	if err != nil {
		return fmt.Errorf("search: listar reseñas para reindexar: %w", err)
	}
	for _, review := range reviews {
		if err := eng.IndexReview(ctx, review.Review); err != nil {
			return err
		}
	}
	return nil
}

func hydrateBooks(ctx context.Context, st *store.Store, ids []int64, page store.Page) ([]store.BookWithAuthor, store.Page, error) {
	page.Total = len(ids)
	if page.Total == 0 {
		return nil, page, nil
	}

	start := page.Offset()
	if start >= len(ids) {
		return nil, page, nil
	}
	end := start + page.Size
	if end > len(ids) {
		end = len(ids)
	}

	out := make([]store.BookWithAuthor, 0, end-start)
	for _, id := range ids[start:end] {
		book, err := st.Books.Get(ctx, id)
		if errors.Is(err, store.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, page, err
		}
		out = append(out, book)
	}
	return out, page, nil
}
