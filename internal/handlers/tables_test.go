package handlers

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"bookreviews/internal/database"
	"bookreviews/internal/models"
	"bookreviews/internal/store"
)

// newTestHandler arma el handler completo sobre una base temporal con un par de
// autores, para poder pegarle a las rutas como lo haría un navegador.
func newTestHandler(t *testing.T) http.Handler {
	t.Helper()
	ctx := context.Background()

	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("abrir base: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := database.Migrate(ctx, db); err != nil {
		t.Fatalf("migrar: %v", err)
	}

	st := store.New(db)
	author := models.Author{Name: "Autora de prueba", CountryOfOrigin: "Chile"}
	if err := st.Authors.Create(ctx, &author); err != nil {
		t.Fatalf("crear autor: %v", err)
	}
	book := models.Book{AuthorID: author.ID, Name: "Libro", PublicationDate: "2010-01-01"}
	if err := st.Books.Create(ctx, &book); err != nil {
		t.Fatalf("crear libro: %v", err)
	}
	if err := st.Reviews.Create(ctx, &models.Review{BookID: book.ID, Text: "Buena", Score: 4}); err != nil {
		t.Fatalf("crear reseña: %v", err)
	}

	handler, err := New(slog.New(slog.NewTextHandler(io.Discard, nil)), st)
	if err != nil {
		t.Fatalf("construir handler: %v", err)
	}
	return handler.Routes()
}

// Un filtro numérico con texto no debe tumbar la página: se ignora y se avisa.
func TestAuthorStatsConFiltrosNoNumericos(t *testing.T) {
	routes := newTestHandler(t)

	queries := []string{
		"", "name=a",
		"min_books=tres", "max_books=tres",
		"min_score=tres", "max_score=tres",
		"min_sales=tres", "max_sales=tres",
		"min_books=x&max_books=y&min_score=z",
		"min_books=2", "sort=total_sales&dir=desc", "sort=zzz&dir=zzz",
	}

	for _, query := range queries {
		t.Run(query, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/authors/stats?"+query, nil)
			rec := httptest.NewRecorder()
			routes.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("status = %d, se esperaba 200. Cuerpo: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestTablasDeReporteResponden(t *testing.T) {
	routes := newTestHandler(t)

	for _, path := range []string{"/authors/stats", "/books/top-rated", "/books/top-selling"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		routes.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("GET %s = %d, se esperaba 200. Cuerpo: %s", path, rec.Code, rec.Body.String())
		}
	}
}
