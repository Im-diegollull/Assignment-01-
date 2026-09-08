package search

import (
	"context"
	"path/filepath"
	"testing"

	"bookreviews/internal/database"
	"bookreviews/internal/models"
	"bookreviews/internal/store"
)

func newSearchStore(t *testing.T) *store.Store {
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
	return store.New(db)
}

func TestSQLFallbackBuscaSoloEnResumen(t *testing.T) {
	st := newSearchStore(t)
	ctx := context.Background()

	author := models.Author{Name: "Ada"}
	if err := st.Authors.Create(ctx, &author); err != nil {
		t.Fatal(err)
	}
	book := models.Book{AuthorID: author.ID, Name: "Naufragio", Summary: "una isla remota", PublicationDate: "2000-01-01"}
	if err := st.Books.Create(ctx, &book); err != nil {
		t.Fatal(err)
	}
	review := models.Review{BookID: book.ID, Text: "me encantó el faro", Score: 5}
	if err := st.Reviews.Create(ctx, &review); err != nil {
		t.Fatal(err)
	}

	sqlEngine := SQL{Store: st}
	page := store.NewPage(1, 10)

	hits, page, err := sqlEngine.Search(ctx, []string{"isla"}, page)
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || len(hits) != 1 {
		t.Fatalf("resumen: total=%d hits=%d", page.Total, len(hits))
	}

	hits, page, err = sqlEngine.Search(ctx, []string{"faro"}, store.NewPage(1, 10))
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 0 {
		t.Fatalf("el fallback no debería encontrar el texto de la reseña, total=%d", page.Total)
	}
}

func TestMemoryEncuentraTituloYResena(t *testing.T) {
	st := newSearchStore(t)
	ctx := context.Background()

	author := models.Author{Name: "Ada"}
	if err := st.Authors.Create(ctx, &author); err != nil {
		t.Fatal(err)
	}
	book := models.Book{AuthorID: author.ID, Name: "Naufragio", Summary: "una isla remota", PublicationDate: "2000-01-01"}
	if err := st.Books.Create(ctx, &book); err != nil {
		t.Fatal(err)
	}
	review := models.Review{BookID: book.ID, Text: "me encantó el faro", Score: 5}
	if err := st.Reviews.Create(ctx, &review); err != nil {
		t.Fatal(err)
	}

	mem := NewMemory(st)
	if err := mem.IndexBook(ctx, book); err != nil {
		t.Fatal(err)
	}
	if err := mem.IndexReview(ctx, review); err != nil {
		t.Fatal(err)
	}

	hits, page, err := mem.Search(ctx, []string{"faro"}, store.NewPage(1, 10))
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || len(hits) != 1 || hits[0].ID != book.ID {
		t.Fatalf("debería encontrar el libro por la reseña: total=%d hits=%d", page.Total, len(hits))
	}

	if err := mem.DeleteReview(ctx, review.ID); err != nil {
		t.Fatal(err)
	}
	hits, page, err = mem.Search(ctx, []string{"faro"}, store.NewPage(1, 10))
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 0 {
		t.Fatalf("después de borrar la reseña del índice, total=%d", page.Total)
	}
}
