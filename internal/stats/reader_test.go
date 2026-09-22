package stats

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"bookreviews/internal/cache"
	"bookreviews/internal/database"
	"bookreviews/internal/models"
	"bookreviews/internal/store"
)

func newReader(t *testing.T) (*Reader, *cache.Memory, *store.Store) {
	t.Helper()
	ctx := context.Background()

	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := database.Migrate(ctx, db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	st := store.New(db)
	mem := cache.NewMemory()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return New(st, mem, logger), mem, st
}

func seedTiny(t *testing.T, st *store.Store) (author models.Author, book models.Book, review models.Review) {
	t.Helper()
	ctx := context.Background()
	author = models.Author{Name: "Ada"}
	if err := st.Authors.Create(ctx, &author); err != nil {
		t.Fatalf("autor: %v", err)
	}
	book = models.Book{AuthorID: author.ID, Name: "Libro", PublicationDate: "2000-01-01"}
	if err := st.Books.Create(ctx, &book); err != nil {
		t.Fatalf("libro: %v", err)
	}
	review = models.Review{BookID: book.ID, Text: "bueno", Score: 4}
	if err := st.Reviews.Create(ctx, &review); err != nil {
		t.Fatalf("reseña: %v", err)
	}
	sale := models.Sale{BookID: book.ID, Year: 2000, Sales: 10}
	if err := st.Sales.Create(ctx, &sale); err != nil {
		t.Fatalf("venta: %v", err)
	}
	return author, book, review
}

func TestMissLlenaElCache(t *testing.T) {
	r, mem, st := newReader(t)
	seedTiny(t, st)
	ctx := context.Background()

	if mem.Has(KeyAuthorsOverview) {
		t.Fatal("el cache debería estar vacío")
	}
	rows, from, err := r.AuthorStats(ctx, store.AuthorFilter{})
	if err != nil {
		t.Fatalf("AuthorStats: %v", err)
	}
	if from != "SQLite" {
		t.Fatalf("primer miss from=%q, se esperaba SQLite", from)
	}
	if len(rows) != 1 {
		t.Fatalf("filas = %d, se esperaba 1", len(rows))
	}
	if !mem.Has(KeyAuthorsOverview) {
		t.Fatal("después del miss debería haberse llenado authors:overview")
	}

	if _, _, err := r.TopRated(ctx, 10); err != nil {
		t.Fatalf("TopRated: %v", err)
	}
	if !mem.Has(KeyTopRated) {
		t.Fatal("falta books:top_rated")
	}
	if _, _, err := r.TopSelling(ctx, 50); err != nil {
		t.Fatalf("TopSelling: %v", err)
	}
	if !mem.Has(KeyTopSelling) {
		t.Fatal("falta books:top_selling")
	}
}

func TestHitNoVuelveALaBase(t *testing.T) {
	r, mem, st := newReader(t)
	_, book, review := seedTiny(t, st)
	ctx := context.Background()

	if _, from, err := r.TopRated(ctx, 10); err != nil {
		t.Fatalf("primera lectura: %v", err)
	} else if from != "SQLite" {
		t.Fatalf("primera lectura from=%q, se esperaba SQLite", from)
	}
	if !mem.Has(BookAvgKey(book.ID)) {
		t.Fatalf("el miss del top 10 debería haber guardado %s", BookAvgKey(book.ID))
	}

	// Si el cache se usa de verdad, borrar la reseña en SQL no cambia el resultado hasta invalidar.
	if err := st.Reviews.Delete(ctx, review.ID); err != nil {
		t.Fatalf("borrar reseña: %v", err)
	}
	rows, from, err := r.TopRated(ctx, 10)
	if err != nil {
		t.Fatalf("hit: %v", err)
	}
	if from != "memory" {
		t.Fatalf("hit from=%q, se esperaba memory", from)
	}
	if len(rows) != 1 {
		t.Fatalf("el hit devolvió %d filas; el cache stale debería seguir mostrando 1", len(rows))
	}

	if err := r.InvalidateReview(ctx, book.ID); err != nil {
		t.Fatalf("invalidar: %v", err)
	}
	if mem.Has(KeyTopRated) || mem.Has(BookAvgKey(book.ID)) || mem.Has(KeyAuthorsOverview) {
		t.Fatal("InvalidateReview debería haber borrado avg, top 10 y overview")
	}

	rows, _, err = r.TopRated(ctx, 10)
	if err != nil {
		t.Fatalf("después de invalidar: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("después de invalidar devolvió %d libros, se esperaba 0", len(rows))
	}
}

func TestInvalidateSale(t *testing.T) {
	r, mem, st := newReader(t)
	seedTiny(t, st)
	ctx := context.Background()

	if _, _, err := r.TopSelling(ctx, 50); err != nil {
		t.Fatal(err)
	}
	if _, _, err := r.AuthorStats(ctx, store.AuthorFilter{}); err != nil {
		t.Fatal(err)
	}
	if err := r.InvalidateSale(ctx); err != nil {
		t.Fatal(err)
	}
	if mem.Has(KeyTopSelling) || mem.Has(KeyAuthorsOverview) {
		t.Fatal("InvalidateSale debería borrar top 50 y overview")
	}
	if mem.Has(KeyTopRated) {
		t.Fatal("una venta no debería tocar el top 10")
	}
}
