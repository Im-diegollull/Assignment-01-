package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"bookreviews/internal/models"
	"bookreviews/internal/store"
)

const catalogIndex = "catalog"

type OpenSearch struct {
	base   string
	client *http.Client
	store  *store.Store
}

func Open(ctx context.Context, url string, st *store.Store) (*OpenSearch, error) {
	os := &OpenSearch{
		base:   strings.TrimRight(url, "/"),
		client: &http.Client{Timeout: 10 * time.Second},
		store:  st,
	}
	if _, err := os.do(ctx, http.MethodGet, "/", nil); err != nil {
		return nil, fmt.Errorf("search: conectar a OpenSearch en %s: %w", url, err)
	}
	if err := os.ensureIndex(ctx); err != nil {
		return nil, err
	}
	return os, nil
}

func (*OpenSearch) Name() string { return "OpenSearch" }

func (o *OpenSearch) ensureIndex(ctx context.Context) error {
	_, status, err := o.doStatus(ctx, http.MethodGet, "/"+catalogIndex, nil)
	if err != nil {
		return err
	}
	if status == http.StatusOK {
		return nil
	}
	mapping := map[string]any{
		"settings": map[string]any{
			"number_of_shards":   1,
			"number_of_replicas": 0,
		},
		"mappings": map[string]any{
			"properties": map[string]any{
				"kind":      map[string]any{"type": "keyword"},
				"book_id":   map[string]any{"type": "long"},
				"review_id": map[string]any{"type": "long"},
				"title":     map[string]any{"type": "text"},
				"summary":   map[string]any{"type": "text"},
				"text":      map[string]any{"type": "text"},
			},
		},
	}
	_, err = o.do(ctx, http.MethodPut, "/"+catalogIndex, mapping)
	return err
}

func (o *OpenSearch) IndexBook(ctx context.Context, book models.Book) error {
	doc := map[string]any{
		"kind":    "book",
		"book_id": book.ID,
		"title":   book.Name,
		"summary": book.Summary,
	}
	_, err := o.do(ctx, http.MethodPut, fmt.Sprintf("/%s/_doc/%s?refresh=true", catalogIndex, bookDocID(book.ID)), doc)
	return err
}

func (o *OpenSearch) DeleteBook(ctx context.Context, bookID int64) error {
	if _, err := o.do(ctx, http.MethodDelete, fmt.Sprintf("/%s/_doc/%s?refresh=true", catalogIndex, bookDocID(bookID)), nil); err != nil {
		return err
	}
	query := map[string]any{
		"query": map[string]any{
			"term": map[string]any{"book_id": bookID},
		},
	}
	_, err := o.do(ctx, http.MethodPost, "/"+catalogIndex+"/_delete_by_query?refresh=true", query)
	return err
}

func (o *OpenSearch) IndexReview(ctx context.Context, review models.Review) error {
	doc := map[string]any{
		"kind":      "review",
		"review_id": review.ID,
		"book_id":   review.BookID,
		"text":      review.Text,
	}
	_, err := o.do(ctx, http.MethodPut, fmt.Sprintf("/%s/_doc/%s?refresh=true", catalogIndex, reviewDocID(review.ID)), doc)
	return err
}

func (o *OpenSearch) DeleteReview(ctx context.Context, reviewID int64) error {
	_, err := o.do(ctx, http.MethodDelete, fmt.Sprintf("/%s/_doc/%s?refresh=true", catalogIndex, reviewDocID(reviewID)), nil)
	return err
}

func (o *OpenSearch) ReindexAll(ctx context.Context) error {
	_, status, err := o.doStatus(ctx, http.MethodDelete, "/"+catalogIndex, nil)
	if err != nil && status != http.StatusNotFound {
		return err
	}
	if err := o.ensureIndex(ctx); err != nil {
		return err
	}
	return reindexFromStore(ctx, o, o.store)
}

func (o *OpenSearch) Search(ctx context.Context, terms []string, page store.Page) ([]store.BookWithAuthor, store.Page, error) {
	page.Total = 0
	if len(terms) == 0 {
		return nil, page, nil
	}

	body := map[string]any{
		"size": 500,
		"query": map[string]any{
			"multi_match": map[string]any{
				"query":    strings.Join(terms, " "),
				"fields":   []string{"title^2", "summary", "text"},
				"operator": "or",
			},
		},
		"collapse": map[string]any{"field": "book_id"},
		"_source":  []string{"book_id"},
	}
	raw, err := o.do(ctx, http.MethodPost, "/"+catalogIndex+"/_search", body)
	if err != nil {
		return nil, page, err
	}

	var parsed struct {
		Hits struct {
			Hits []struct {
				Source struct {
					BookID int64 `json:"book_id"`
				} `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, page, fmt.Errorf("search: decodificar hits: %w", err)
	}

	ids := make([]int64, 0, len(parsed.Hits.Hits))
	for _, hit := range parsed.Hits.Hits {
		if hit.Source.BookID > 0 {
			ids = append(ids, hit.Source.BookID)
		}
	}
	return hydrateBooks(ctx, o.store, ids, page)
}

func (o *OpenSearch) do(ctx context.Context, method, path string, body any) ([]byte, error) {
	raw, status, err := o.doStatus(ctx, method, path, body)
	if err != nil {
		return nil, err
	}
	if method == http.MethodDelete && status == http.StatusNotFound {
		return raw, nil
	}
	if status >= 300 {
		return nil, fmt.Errorf("search: %s %s → %d: %s", method, path, status, truncate(raw, 300))
	}
	return raw, nil
}

func (o *OpenSearch) doStatus(ctx context.Context, method, path string, body any) ([]byte, int, error) {
	var rdr io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		rdr = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, o.base+path, rdr)
	if err != nil {
		return nil, 0, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := o.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, res.StatusCode, err
	}
	return raw, res.StatusCode, nil
}

func bookDocID(id int64) string   { return fmt.Sprintf("book-%d", id) }
func reviewDocID(id int64) string { return fmt.Sprintf("review-%d", id) }

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "…"
}
