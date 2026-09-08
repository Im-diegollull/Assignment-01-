package stats

import "context"


func (r *Reader) InvalidateReview(ctx context.Context, bookIDs ...int64) error {
	keys := []string{KeyTopRated, KeyAuthorsOverview}
	for _, id := range bookIDs {
		if id > 0 {
			keys = append(keys, BookAvgKey(id))
		}
	}
	return r.cache.Del(ctx, unique(keys)...)
}

// InvalidateSale borra el top 50 y las ventas del overview de autores.
func (r *Reader) InvalidateSale(ctx context.Context) error {
	return r.cache.Del(ctx, KeyTopSelling, KeyAuthorsOverview)
}


func (r *Reader) InvalidateCatalog(ctx context.Context) error {
	return r.cache.Del(ctx, KeyAuthorsOverview, KeyTopRated, KeyTopSelling)
}

func (r *Reader) InvalidateBookRemoved(ctx context.Context, bookIDs ...int64) error {
	keys := []string{KeyAuthorsOverview, KeyTopRated, KeyTopSelling}
	for _, id := range bookIDs {
		if id > 0 {
			keys = append(keys, BookAvgKey(id))
		}
	}
	return r.cache.Del(ctx, unique(keys)...)
}

func (r *Reader) Flush(ctx context.Context) error {
	return r.cache.Flush(ctx)
}

func unique(keys []string) []string {
	seen := make(map[string]struct{}, len(keys))
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	return out
}
