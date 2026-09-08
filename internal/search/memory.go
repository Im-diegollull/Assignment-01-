package search

import (
	"context"
	"strings"
	"sync"

	"bookreviews/internal/models"
	"bookreviews/internal/store"
)

type Memory struct {
	Store   *store.Store
	mu      sync.Mutex
	books   map[int64]models.Book
	reviews map[int64]models.Review
}

func NewMemory(st *store.Store) *Memory {
	return &Memory{
		Store:   st,
		books:   make(map[int64]models.Book),
		reviews: make(map[int64]models.Review),
	}
}

func (*Memory) Name() string { return "memory" }

func (m *Memory) IndexBook(_ context.Context, book models.Book) error {
	m.mu.Lock()
	m.books[book.ID] = book
	m.mu.Unlock()
	return nil
}

func (m *Memory) DeleteBook(_ context.Context, bookID int64) error {
	m.mu.Lock()
	delete(m.books, bookID)
	for id, review := range m.reviews {
		if review.BookID == bookID {
			delete(m.reviews, id)
		}
	}
	m.mu.Unlock()
	return nil
}

func (m *Memory) IndexReview(_ context.Context, review models.Review) error {
	m.mu.Lock()
	m.reviews[review.ID] = review
	m.mu.Unlock()
	return nil
}

func (m *Memory) DeleteReview(_ context.Context, reviewID int64) error {
	m.mu.Lock()
	delete(m.reviews, reviewID)
	m.mu.Unlock()
	return nil
}

func (m *Memory) ReindexAll(ctx context.Context) error {
	return reindexFromStore(ctx, m, m.Store)
}

func (m *Memory) Search(ctx context.Context, terms []string, page store.Page) ([]store.BookWithAuthor, store.Page, error) {
	page.Total = 0
	if len(terms) == 0 {
		return nil, page, nil
	}

	m.mu.Lock()
	ids := make([]int64, 0)
	seen := make(map[int64]struct{})
	for _, book := range m.books {
		if matchesAny(terms, book.Name, book.Summary) {
			ids = append(ids, book.ID)
			seen[book.ID] = struct{}{}
		}
	}
	for _, review := range m.reviews {
		if _, ok := seen[review.BookID]; ok {
			continue
		}
		if matchesAny(terms, review.Text) {
			ids = append(ids, review.BookID)
			seen[review.BookID] = struct{}{}
		}
	}
	m.mu.Unlock()

	return hydrateBooks(ctx, m.Store, ids, page)
}

func matchesAny(terms []string, fields ...string) bool {
	blob := strings.ToLower(strings.Join(fields, " "))
	for _, term := range terms {
		if strings.Contains(blob, strings.ToLower(term)) {
			return true
		}
	}
	return false
}
