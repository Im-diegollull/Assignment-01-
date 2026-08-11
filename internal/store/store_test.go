package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"bookreviews/internal/database"
	"bookreviews/internal/models"
)

// newTestStore devuelve un Store sobre una base temporal ya migrada.
func newTestStore(t *testing.T) *Store {
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
	return New(db)
}

func mustCreateAuthor(t *testing.T, s *Store, name string) models.Author {
	t.Helper()

	author := models.Author{Name: name, CountryOfOrigin: "Chile"}
	if err := s.Authors.Create(context.Background(), &author); err != nil {
		t.Fatalf("crear autor: %v", err)
	}
	return author
}

func mustCreateBook(t *testing.T, s *Store, authorID int64, name string) models.Book {
	t.Helper()

	book := models.Book{
		AuthorID:        authorID,
		Name:            name,
		Summary:         "Resumen de prueba",
		PublicationDate: "2015-06-01",
	}
	if err := s.Books.Create(context.Background(), &book); err != nil {
		t.Fatalf("crear libro: %v", err)
	}
	return book
}

// bookSales lee el campo denormalizado directo de la base.
func bookSales(t *testing.T, s *Store, bookID int64) int {
	t.Helper()

	book, err := s.Books.Get(context.Background(), bookID)
	if err != nil {
		t.Fatalf("obtener libro %d: %v", bookID, err)
	}
	return book.NumberOfSales
}

func TestAuthorCRUD(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	author := models.Author{
		Name:            "Gabriela Mistral",
		DateOfBirth:     "1889-04-07",
		CountryOfOrigin: "Chile",
		Description:     "Poeta y diplomática.",
	}
	if err := s.Authors.Create(ctx, &author); err != nil {
		t.Fatalf("crear: %v", err)
	}
	if author.ID == 0 {
		t.Fatal("Create no completó el ID")
	}

	got, err := s.Authors.Get(ctx, author.ID)
	if err != nil {
		t.Fatalf("obtener: %v", err)
	}
	if got != author {
		t.Errorf("obtenido %+v, se esperaba %+v", got, author)
	}

	author.Name = "Lucila Godoy"
	if err := s.Authors.Update(ctx, &author); err != nil {
		t.Fatalf("actualizar: %v", err)
	}
	if got, _ := s.Authors.Get(ctx, author.ID); got.Name != "Lucila Godoy" {
		t.Errorf("nombre = %q tras actualizar", got.Name)
	}

	if err := s.Authors.Delete(ctx, author.ID); err != nil {
		t.Fatalf("borrar: %v", err)
	}
	if _, err := s.Authors.Get(ctx, author.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("tras borrar, Get devolvió %v; se esperaba ErrNotFound", err)
	}
}

// Un id inexistente tiene que dar ErrNotFound en las tres operaciones, para que
// el handler pueda responder 404 sin distinguir casos.
func TestOperacionesSobreIDInexistente(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	if _, err := s.Authors.Get(ctx, 404); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get: %v", err)
	}
	if err := s.Authors.Update(ctx, &models.Author{ID: 404, Name: "X"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("Update: %v", err)
	}
	if err := s.Authors.Delete(ctx, 404); !errors.Is(err, ErrNotFound) {
		t.Errorf("Delete: %v", err)
	}
}

func TestListPaginaYCuenta(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	for _, name := range []string{"Ana", "Bruno", "Carla", "Diego", "Elena"} {
		mustCreateAuthor(t, s, name)
	}

	authors, page, err := s.Authors.List(ctx, NewPage(1, 2))
	if err != nil {
		t.Fatalf("listar: %v", err)
	}
	if len(authors) != 2 {
		t.Errorf("página 1 trajo %d autores, se esperaban 2", len(authors))
	}
	if page.Total != 5 {
		t.Errorf("Total = %d, se esperaba 5", page.Total)
	}
	if page.TotalPages() != 3 {
		t.Errorf("TotalPages = %d, se esperaba 3", page.TotalPages())
	}
	if authors[0].Name != "Ana" {
		t.Errorf("primer autor = %q, se esperaba Ana (orden alfabético)", authors[0].Name)
	}

	last, page, err := s.Authors.List(ctx, NewPage(3, 2))
	if err != nil {
		t.Fatalf("listar página 3: %v", err)
	}
	if len(last) != 1 {
		t.Errorf("última página trajo %d autores, se esperaba 1", len(last))
	}
	if page.HasNext() {
		t.Error("la última página no debería tener siguiente")
	}
}

// Núcleo de la fase: books.number_of_sales tiene que seguir a sales_by_year.
func TestVentasRecalculanElTotalDelLibro(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	book := mustCreateBook(t, s, mustCreateAuthor(t, s, "Autora").ID, "Libro")

	first := models.Sale{BookID: book.ID, Year: 2020, Sales: 100}
	if err := s.Sales.Create(ctx, &first); err != nil {
		t.Fatalf("crear venta 2020: %v", err)
	}
	second := models.Sale{BookID: book.ID, Year: 2021, Sales: 250}
	if err := s.Sales.Create(ctx, &second); err != nil {
		t.Fatalf("crear venta 2021: %v", err)
	}
	if got := bookSales(t, s, book.ID); got != 350 {
		t.Errorf("tras dos altas number_of_sales = %d, se esperaba 350", got)
	}

	second.Sales = 50
	if err := s.Sales.Update(ctx, &second); err != nil {
		t.Fatalf("actualizar venta: %v", err)
	}
	if got := bookSales(t, s, book.ID); got != 150 {
		t.Errorf("tras editar number_of_sales = %d, se esperaba 150", got)
	}

	if err := s.Sales.Delete(ctx, first.ID); err != nil {
		t.Fatalf("borrar venta: %v", err)
	}
	if got := bookSales(t, s, book.ID); got != 50 {
		t.Errorf("tras borrar number_of_sales = %d, se esperaba 50", got)
	}

	// Borrar la última venta deja SUM en NULL: el COALESCE tiene que dar 0.
	if err := s.Sales.Delete(ctx, second.ID); err != nil {
		t.Fatalf("borrar última venta: %v", err)
	}
	if got := bookSales(t, s, book.ID); got != 0 {
		t.Errorf("sin ventas number_of_sales = %d, se esperaba 0", got)
	}
}

// Mover una venta de un libro a otro tiene que recalcular ambos totales.
func TestMoverUnaVentaRecalculaLosDosLibros(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	authorID := mustCreateAuthor(t, s, "Autora").ID
	origin := mustCreateBook(t, s, authorID, "Origen")
	target := mustCreateBook(t, s, authorID, "Destino")

	sale := models.Sale{BookID: origin.ID, Year: 2019, Sales: 500}
	if err := s.Sales.Create(ctx, &sale); err != nil {
		t.Fatalf("crear venta: %v", err)
	}

	sale.BookID = target.ID
	if err := s.Sales.Update(ctx, &sale); err != nil {
		t.Fatalf("mover venta: %v", err)
	}

	if got := bookSales(t, s, origin.ID); got != 0 {
		t.Errorf("el libro de origen quedó con %d ventas, se esperaba 0", got)
	}
	if got := bookSales(t, s, target.ID); got != 500 {
		t.Errorf("el libro de destino quedó con %d ventas, se esperaba 500", got)
	}
}

func TestVentaDuplicadaParaElMismoAño(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	book := mustCreateBook(t, s, mustCreateAuthor(t, s, "Autora").ID, "Libro")

	first := models.Sale{BookID: book.ID, Year: 2020, Sales: 10}
	if err := s.Sales.Create(ctx, &first); err != nil {
		t.Fatalf("primera venta: %v", err)
	}

	duplicate := models.Sale{BookID: book.ID, Year: 2020, Sales: 99}
	if err := s.Sales.Create(ctx, &duplicate); !errors.Is(err, ErrDuplicateSaleYear) {
		t.Errorf("crear duplicada devolvió %v; se esperaba ErrDuplicateSaleYear", err)
	}

	// El total no debe haberse movido: la transacción hizo rollback.
	if got := bookSales(t, s, book.ID); got != 10 {
		t.Errorf("number_of_sales = %d tras el intento fallido, se esperaba 10", got)
	}

	// Editar una venta sin cambiarle el año no debe chocar consigo misma.
	first.Sales = 20
	if err := s.Sales.Update(ctx, &first); err != nil {
		t.Errorf("editar la propia venta falló con %v", err)
	}
}

func TestBorrarLibroArrastraReseñasYVentas(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	book := mustCreateBook(t, s, mustCreateAuthor(t, s, "Autora").ID, "Libro")

	review := models.Review{BookID: book.ID, Text: "Buena", Score: 4}
	if err := s.Reviews.Create(ctx, &review); err != nil {
		t.Fatalf("crear reseña: %v", err)
	}
	sale := models.Sale{BookID: book.ID, Year: 2020, Sales: 10}
	if err := s.Sales.Create(ctx, &sale); err != nil {
		t.Fatalf("crear venta: %v", err)
	}

	if err := s.Books.Delete(ctx, book.ID); err != nil {
		t.Fatalf("borrar libro: %v", err)
	}

	reviews, err := s.Reviews.ListByBook(ctx, book.ID)
	if err != nil {
		t.Fatalf("listar reseñas: %v", err)
	}
	if len(reviews) != 0 {
		t.Errorf("quedaron %d reseñas tras borrar el libro", len(reviews))
	}

	sales, err := s.Sales.ListByBook(ctx, book.ID)
	if err != nil {
		t.Fatalf("listar ventas: %v", err)
	}
	if len(sales) != 0 {
		t.Errorf("quedaron %d ventas tras borrar el libro", len(sales))
	}
}

func TestReviewCRUD(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	book := mustCreateBook(t, s, mustCreateAuthor(t, s, "Autora").ID, "Libro")

	review := models.Review{BookID: book.ID, Text: "Excelente", Score: 5, Upvotes: 3}
	if err := s.Reviews.Create(ctx, &review); err != nil {
		t.Fatalf("crear: %v", err)
	}

	got, err := s.Reviews.Get(ctx, review.ID)
	if err != nil {
		t.Fatalf("obtener: %v", err)
	}
	if got.Score != 5 || got.Upvotes != 3 || got.BookName != "Libro" {
		t.Errorf("obtenido %+v", got)
	}

	review.Score = 2
	if err := s.Reviews.Update(ctx, &review); err != nil {
		t.Fatalf("actualizar: %v", err)
	}
	if got, _ := s.Reviews.Get(ctx, review.ID); got.Score != 2 {
		t.Errorf("score = %d tras actualizar", got.Score)
	}

	if err := s.Reviews.Delete(ctx, review.ID); err != nil {
		t.Fatalf("borrar: %v", err)
	}
	if _, err := s.Reviews.Get(ctx, review.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("tras borrar: %v", err)
	}
}
