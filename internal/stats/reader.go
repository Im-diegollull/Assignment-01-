// Package stats envuelve las queries caras de reportes con un cache opcional. store sigue siendo la fuente de verdad
package stats

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"

	"bookreviews/internal/cache"
	"bookreviews/internal/store"
)

type Reader struct {
	store  *store.Store
	cache  cache.Client
	logger *slog.Logger
}

func New(st *store.Store, c cache.Client, logger *slog.Logger) *Reader {
	if c == nil {
		c = cache.Noop()
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Reader{store: st, cache: c, logger: logger}
}

func (r *Reader) CacheName() string { return r.cache.Name() }

func (r *Reader) AuthorStats(ctx context.Context, filter store.AuthorFilter) ([]store.AuthorStatsRow, string, error) {
	rows, from, err := getOrLoad(ctx, r, KeyAuthorsOverview, func(ctx context.Context) ([]store.AuthorStatsRow, error) {
		return r.store.Authors.AuthorStatsAll(ctx)
	})
	if err != nil {
		return nil, from, err
	}
	return store.ApplyAuthorFilter(rows, filter), from, nil
}

func (r *Reader) TopRated(ctx context.Context, limit int) ([]store.TopRatedBookRow, string, error) {
	rows, from, err := getOrLoad(ctx, r, KeyTopRated, func(ctx context.Context) ([]store.TopRatedBookRow, error) {
		return r.store.Books.TopRated(ctx, limit)
	})
	if err != nil {
		return nil, from, err
	}
	for _, row := range rows {
		avg := sql.NullFloat64{Float64: row.AvgScore, Valid: true}
		if err := r.setJSON(ctx, BookAvgKey(row.BookID), avg); err != nil {
			r.logger.Error("cache: guardar promedio de libro", "book_id", row.BookID, "error", err)
		}
	}
	return rows, from, nil
}

func (r *Reader) TopSelling(ctx context.Context, limit int) ([]store.TopSellingBookRow, string, error) {
	return getOrLoad(ctx, r, KeyTopSelling, func(ctx context.Context) ([]store.TopSellingBookRow, error) {
		return r.store.Books.TopSelling(ctx, limit)
	})
}

func (r *Reader) BookAverage(ctx context.Context, bookID int64) (sql.NullFloat64, string, error) {
	return getOrLoad(ctx, r, BookAvgKey(bookID), func(ctx context.Context) (sql.NullFloat64, error) {
		return r.store.Books.AverageScore(ctx, bookID)
	})
}

func getOrLoad[T any](ctx context.Context, r *Reader, key string, load func(context.Context) (T, error)) (T, string, error) {
	var zero T
	if raw, ok, err := r.cache.Get(ctx, key); err != nil {
		r.logger.Error("cache: GET", "key", key, "error", err)
	} else if ok {
		var value T
		if err := json.Unmarshal(raw, &value); err != nil {
			r.logger.Error("cache: decodificar", "key", key, "error", err)
		} else {
			r.logger.Info("cache hit", "key", key, "from", r.hitLabel())
			return value, r.hitLabel(), nil
		}
	}

	value, err := load(ctx)
	if err != nil {
		return zero, "SQLite", err
	}
	if err := r.setJSON(ctx, key, value); err != nil {
		r.logger.Error("cache: SET", "key", key, "error", err)
	}
	r.logger.Info("cache miss", "key", key, "from", "SQLite")
	return value, "SQLite", nil
}

func (r *Reader) hitLabel() string {
	if name := r.cache.Name(); name != "off" {
		return name
	}
	return "SQLite"
}

func (r *Reader) setJSON(ctx context.Context, key string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return r.cache.Set(ctx, key, raw)
}
