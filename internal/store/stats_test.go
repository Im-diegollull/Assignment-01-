package store

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"bookreviews/internal/models"
)

type statsFixture struct {
	alba, bruno, cora, dante models.Author
	uno, dos, tres, cuatro   models.Book
}

func newStatsFixture(t *testing.T, s *Store) statsFixture {
	t.Helper()
	ctx := context.Background()

	var f statsFixture
	f.alba = mustCreateAuthor(t, s, "Alba")
	f.bruno = mustCreateAuthor(t, s, "Bruno")
	f.cora = mustCreateAuthor(t, s, "Cora")
	f.dante = mustCreateAuthor(t, s, "Dante")

	newBook := func(author models.Author, name, pubDate string) models.Book {
		book := models.Book{AuthorID: author.ID, Name: name, PublicationDate: pubDate}
		if err := s.Books.Create(ctx, &book); err != nil {
			t.Fatalf("crear libro %s: %v", name, err)
		}
		return book
	}
	f.uno = newBook(f.alba, "Uno", "2000-05-01")
	f.dos = newBook(f.alba, "Dos", "2001-05-01")
	f.tres = newBook(f.bruno, "Tres", "2000-05-01")
	f.cuatro = newBook(f.cora, "Cuatro", "2001-05-01")

	addReview := func(book models.Book, score, upvotes int, text string) {
		review := models.Review{BookID: book.ID, Text: text, Score: score, Upvotes: upvotes}
		if err := s.Reviews.Create(ctx, &review); err != nil {
			t.Fatalf("crear reseña: %v", err)
		}
	}
	addReview(f.uno, 5, 10, "excelente con muchos votos")
	addReview(f.uno, 5, 3, "excelente con pocos votos")
	addReview(f.uno, 4, 7, "muy bueno")
	addReview(f.dos, 2, 4, "flojo")
	addReview(f.dos, 4, 1, "aceptable")
	addReview(f.tres, 5, 2, "unica reseña")

	addSale := func(book models.Book, year, units int) {
		sale := models.Sale{BookID: book.ID, Year: year, Sales: units}
		if err := s.Sales.Create(ctx, &sale); err != nil {
			t.Fatalf("crear venta: %v", err)
		}
	}
	addSale(f.uno, 2000, 100)
	addSale(f.uno, 2001, 50)
	addSale(f.dos, 2001, 900)
	addSale(f.tres, 2000, 300)
	addSale(f.cuatro, 2001, 20)

	return f
}

// ---------------------------------------------------------------------------
// 5.1 Tabla de autores
// ---------------------------------------------------------------------------

func TestAuthorStatsAgregaSinFanOut(t *testing.T) {
	s := newTestStore(t)
	newStatsFixture(t, s)

	rows, err := s.Authors.AuthorStats(context.Background(), AuthorFilter{SortBy: "name"})
	if err != nil {
		t.Fatalf("AuthorStats: %v", err)
	}
	if len(rows) != 4 {
		t.Fatalf("devolvió %d filas, se esperaban 4 (incluido el autor sin libros)", len(rows))
	}

	want := []struct {
		name       string
		books      int
		hasScore   bool
		avgScore   float64
		totalSales int
	}{

		{"Alba", 2, true, 4.0, 1050},
		{"Bruno", 1, true, 5.0, 300},
		{"Cora", 1, false, 0, 20},
		{"Dante", 0, false, 0, 0},
	}

	for i, w := range want {
		got := rows[i]
		if got.Name != w.name {
			t.Errorf("fila %d: nombre = %q, se esperaba %q", i, got.Name, w.name)
			continue
		}
		if got.BooksCount != w.books {
			t.Errorf("%s: libros = %d, se esperaban %d", w.name, got.BooksCount, w.books)
		}
		if got.TotalSales != w.totalSales {
			t.Errorf("%s: ventas = %d, se esperaban %d", w.name, got.TotalSales, w.totalSales)
		}
		if got.AvgScore.Valid != w.hasScore {
			t.Errorf("%s: AvgScore.Valid = %v, se esperaba %v", w.name, got.AvgScore.Valid, w.hasScore)
		}
		if w.hasScore && !almostEqual(got.AvgScore.Float64, w.avgScore) {
			t.Errorf("%s: promedio = %v, se esperaba %v", w.name, got.AvgScore.Float64, w.avgScore)
		}
	}
}

func TestAuthorStatsOrden(t *testing.T) {
	s := newTestStore(t)
	newStatsFixture(t, s)
	ctx := context.Background()

	cases := []struct {
		name   string
		filter AuthorFilter
		want   []string
	}{
		{"por nombre asc", AuthorFilter{SortBy: "name", Dir: "asc"},
			[]string{"Alba", "Bruno", "Cora", "Dante"}},
		{"por nombre desc", AuthorFilter{SortBy: "name", Dir: "desc"},
			[]string{"Dante", "Cora", "Bruno", "Alba"}},
		{"por libros desc", AuthorFilter{SortBy: "books", Dir: "desc"},
			[]string{"Alba", "Bruno", "Cora", "Dante"}},
		{"por ventas desc", AuthorFilter{SortBy: "total_sales", Dir: "desc"},
			[]string{"Alba", "Bruno", "Cora", "Dante"}},
		{"por ventas asc", AuthorFilter{SortBy: "total_sales", Dir: "asc"},
			[]string{"Dante", "Cora", "Bruno", "Alba"}},
		// Con avg_score DESC los NULL (autores sin reseñas) quedan al final.
		{"por score desc", AuthorFilter{SortBy: "avg_score", Dir: "desc"},
			[]string{"Bruno", "Alba", "Cora", "Dante"}},
		// Una columna desconocida no debe llegar al SQL: cae en el default.
		{"columna inválida cae en nombre", AuthorFilter{SortBy: "'; DROP TABLE authors;--"},
			[]string{"Alba", "Bruno", "Cora", "Dante"}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rows, err := s.Authors.AuthorStats(ctx, c.filter)
			if err != nil {
				t.Fatalf("AuthorStats: %v", err)
			}
			if len(rows) != len(c.want) {
				t.Fatalf("devolvió %d filas, se esperaban %d", len(rows), len(c.want))
			}
			for i, name := range c.want {
				if rows[i].Name != name {
					t.Errorf("posición %d: %q, se esperaba %q", i, rows[i].Name, name)
				}
			}
		})
	}
}

func TestAuthorStatsFiltros(t *testing.T) {
	s := newTestStore(t)
	newStatsFixture(t, s)
	ctx := context.Background()

	intPtr := func(v int) *int { return &v }
	floatPtr := func(v float64) *float64 { return &v }

	cases := []struct {
		name   string
		filter AuthorFilter
		want   []string
	}{
		{"nombre contiene 'a'", AuthorFilter{NameLike: "a", SortBy: "name"},
			[]string{"Alba", "Cora", "Dante"}},
		{"nombre exacto", AuthorFilter{NameLike: "Bruno", SortBy: "name"},
			[]string{"Bruno"}},
		{"al menos 1 libro", AuthorFilter{MinBooks: intPtr(1), SortBy: "name"},
			[]string{"Alba", "Bruno", "Cora"}},
		{"al menos 2 libros", AuthorFilter{MinBooks: intPtr(2), SortBy: "name"},
			[]string{"Alba"}},
		{"a lo sumo 1 libro", AuthorFilter{MaxBooks: intPtr(1), SortBy: "name"},
			[]string{"Bruno", "Cora", "Dante"}},
		{"ventas >= 300", AuthorFilter{MinSales: intPtr(300), SortBy: "name"},
			[]string{"Alba", "Bruno"}},
		{"ventas entre 20 y 300", AuthorFilter{MinSales: intPtr(20), MaxSales: intPtr(300), SortBy: "name"},
			[]string{"Bruno", "Cora"}},
		// Cora y Dante no tienen promedio: NULL >= 4 no es verdadero, así que
		// un filtro por score los deja fuera.
		{"score >= 4 excluye a los sin reseñas", AuthorFilter{MinScore: floatPtr(4), SortBy: "name"},
			[]string{"Alba", "Bruno"}},
		{"score >= 4.5", AuthorFilter{MinScore: floatPtr(4.5), SortBy: "name"},
			[]string{"Bruno"}},
		{"combinado: >=1 libro y ventas <= 300", AuthorFilter{
			MinBooks: intPtr(1), MaxSales: intPtr(300), SortBy: "name"},
			[]string{"Bruno", "Cora"}},
		{"filtro sin resultados", AuthorFilter{MinBooks: intPtr(99), SortBy: "name"},
			nil},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rows, err := s.Authors.AuthorStats(ctx, c.filter)
			if err != nil {
				t.Fatalf("AuthorStats: %v", err)
			}
			if len(rows) != len(c.want) {
				t.Fatalf("devolvió %d filas %v, se esperaban %d %v",
					len(rows), names(rows), len(c.want), c.want)
			}
			for i, name := range c.want {
				if rows[i].Name != name {
					t.Errorf("posición %d: %q, se esperaba %q", i, rows[i].Name, name)
				}
			}
		})
	}
}

func TestAuthorFilterConditionsEsPuraYParametrizada(t *testing.T) {
	min, max := 2, 8
	filter := AuthorFilter{NameLike: "  O'Brien  ", MinBooks: &min, MaxSales: &max}

	fragments, args := filter.conditions()
	if len(fragments) != 3 {
		t.Fatalf("devolvió %d fragmentos %v, se esperaban 3", len(fragments), fragments)
	}
	if len(args) != 3 {
		t.Fatalf("devolvió %d argumentos %v, se esperaban 3", len(args), args)
	}

	for _, fragment := range fragments {
		if strings.Count(fragment, "?") != 1 {
			t.Errorf("el fragmento %q no usa exactamente un placeholder", fragment)
		}
		if strings.Contains(fragment, "O'Brien") {
			t.Errorf("el fragmento %q tiene el valor del usuario incrustado en el SQL", fragment)
		}
	}

	if args[0] != "%O'Brien%" {
		t.Errorf("args[0] = %v, se esperaba %q", args[0], "%O'Brien%")
	}
	if args[1] != 2 || args[2] != 8 {
		t.Errorf("args = %v, se esperaban [%%O'Brien%% 2 8]", args)
	}
}

func TestAuthorFilterOrderByRechazaColumnasDesconocidas(t *testing.T) {
	cases := map[string]string{
		"name":                  "a.name COLLATE NOCASE ASC, a.id",
		"total_sales":           "COALESCE(bs.total_sales, 0) ASC, a.id",
		"inventada":             "a.name COLLATE NOCASE ASC, a.id",
		"a.name; DROP TABLE --": "a.name COLLATE NOCASE ASC, a.id",
	}
	for sortBy, want := range cases {
		if got := (AuthorFilter{SortBy: sortBy}).orderBy(); got != want {
			t.Errorf("orderBy(%q) = %q, se esperaba %q", sortBy, got, want)
		}
	}

	// La dirección solo puede terminar en ASC o DESC.
	if got := (AuthorFilter{SortBy: "name", Dir: "DESC"}).orderBy(); got != "a.name COLLATE NOCASE DESC, a.id" {
		t.Errorf("dir DESC dio %q", got)
	}
	if got := (AuthorFilter{SortBy: "name", Dir: "; DROP"}).orderBy(); got != "a.name COLLATE NOCASE ASC, a.id" {
		t.Errorf("una dirección inválida dio %q, se esperaba ASC", got)
	}
}

// ---------------------------------------------------------------------------
// 5.2 Top mejor evaluados
// ---------------------------------------------------------------------------

func TestTopRated(t *testing.T) {
	s := newTestStore(t)
	f := newStatsFixture(t, s)

	rows, err := s.Books.TopRated(context.Background(), 10)
	if err != nil {
		t.Fatalf("TopRated: %v", err)
	}

	// "Cuatro" no tiene reseñas, así que no entra al ranking.
	if len(rows) != 3 {
		t.Fatalf("devolvió %d libros, se esperaban 3 (los que tienen reseñas)", len(rows))
	}

	// Orden esperado: Tres (5.0), Uno (4.667), Dos (3.0).
	if rows[0].BookName != "Tres" || rows[1].BookName != "Uno" || rows[2].BookName != "Dos" {
		t.Fatalf("orden = %v, se esperaba [Tres Uno Dos]", bookNames(rows))
	}

	if !almostEqual(rows[1].AvgScore, 14.0/3.0) {
		t.Errorf("promedio de Uno = %v, se esperaba 4.667", rows[1].AvgScore)
	}
	if rows[1].ReviewsCount != 3 {
		t.Errorf("reseñas de Uno = %d, se esperaban 3", rows[1].ReviewsCount)
	}
	if rows[1].AuthorName != "Alba" {
		t.Errorf("autor de Uno = %q, se esperaba Alba", rows[1].AuthorName)
	}

	// Uno tiene dos reseñas de 5: desempata el que tiene más up-votes.
	if rows[1].BestScore != 5 || rows[1].BestReview != "excelente con muchos votos" {
		t.Errorf("mejor reseña de Uno = (%d, %q)", rows[1].BestScore, rows[1].BestReview)
	}
	if rows[1].WorstScore != 4 || rows[1].WorstReview != "muy bueno" {
		t.Errorf("peor reseña de Uno = (%d, %q)", rows[1].WorstScore, rows[1].WorstReview)
	}

	// Con una sola reseña, la mejor y la peor son la misma: es correcto.
	if rows[0].BestReview != "unica reseña" || rows[0].WorstReview != "unica reseña" {
		t.Errorf("con una sola reseña, mejor = %q y peor = %q",
			rows[0].BestReview, rows[0].WorstReview)
	}
	if rows[0].BookID != f.tres.ID {
		t.Errorf("el primero es el libro %d, se esperaba %d", rows[0].BookID, f.tres.ID)
	}
}

func TestTopRatedRespetaElLimite(t *testing.T) {
	s := newTestStore(t)
	newStatsFixture(t, s)

	rows, err := s.Books.TopRated(context.Background(), 2)
	if err != nil {
		t.Fatalf("TopRated: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("devolvió %d libros, se esperaban 2", len(rows))
	}
	if rows[0].BookName != "Tres" || rows[1].BookName != "Uno" {
		t.Errorf("con límite 2 devolvió %v", bookNames(rows))
	}
}

// ---------------------------------------------------------------------------
// 5.3 Top más vendidos
// ---------------------------------------------------------------------------

func TestTopSelling(t *testing.T) {
	s := newTestStore(t)
	newStatsFixture(t, s)

	rows, err := s.Books.TopSelling(context.Background(), 50)
	if err != nil {
		t.Fatalf("TopSelling: %v", err)
	}
	if len(rows) != 4 {
		t.Fatalf("devolvió %d libros, se esperaban 4", len(rows))
	}

	// Ventas por libro: Dos 900, Tres 300, Uno 150, Cuatro 20.
	want := []struct {
		book        string
		bookSales   int
		authorSales int
		pubYear     int
		inTop5      bool
	}{
		// Las ventas del autor son las de todos sus libros, no las de este.
		{"Dos", 900, 1050, 2001, true},
		{"Tres", 300, 300, 2000, true},
		{"Uno", 150, 1050, 2000, true},
		{"Cuatro", 20, 20, 2001, true},
	}

	for i, w := range want {
		got := rows[i]
		if got.BookName != w.book {
			t.Errorf("posición %d: %q, se esperaba %q", i, got.BookName, w.book)
			continue
		}
		if got.BookSales != w.bookSales {
			t.Errorf("%s: ventas del libro = %d, se esperaban %d", w.book, got.BookSales, w.bookSales)
		}
		if got.AuthorSales != w.authorSales {
			t.Errorf("%s: ventas del autor = %d, se esperaban %d", w.book, got.AuthorSales, w.authorSales)
		}
		if got.PublicationYear != w.pubYear {
			t.Errorf("%s: año = %d, se esperaba %d", w.book, got.PublicationYear, w.pubYear)
		}
		if got.InTop5OfYear != w.inTop5 {
			t.Errorf("%s: top5 del año = %v, se esperaba %v", w.book, got.InTop5OfYear, w.inTop5)
		}
	}
}

// Con menos de 5 libros por año todos entran al top 5; para probar el flag de
// verdad hace falta un año con más de 5 competidores.
func TestTopSellingFlagTop5ConCompetencia(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	author := mustCreateAuthor(t, s, "Prolífica")

	// Siete libros publicados en 2010, con ventas 700, 600, ..., 100 en 2010.
	// Los cinco primeros están en el top 5 de ese año; los dos últimos no.
	type expectation struct {
		id     int64
		inTop5 bool
	}
	var expected []expectation

	for i := range 7 {
		units := 700 - i*100
		book := models.Book{
			AuthorID:        author.ID,
			Name:            fmt.Sprintf("Libro %d", i+1),
			PublicationDate: "2010-01-01",
		}
		if err := s.Books.Create(ctx, &book); err != nil {
			t.Fatalf("crear libro: %v", err)
		}
		sale := models.Sale{BookID: book.ID, Year: 2010, Sales: units}
		if err := s.Sales.Create(ctx, &sale); err != nil {
			t.Fatalf("crear venta: %v", err)
		}
		expected = append(expected, expectation{id: book.ID, inTop5: i < 5})
	}

	rows, err := s.Books.TopSelling(ctx, 50)
	if err != nil {
		t.Fatalf("TopSelling: %v", err)
	}

	flags := make(map[int64]bool, len(rows))
	for _, row := range rows {
		flags[row.BookID] = row.InTop5OfYear
	}
	for i, e := range expected {
		if flags[e.id] != e.inTop5 {
			t.Errorf("Libro %d (ventas %d): top5 = %v, se esperaba %v",
				i+1, 700-i*100, flags[e.id], e.inTop5)
		}
	}
}

// Un libro puede vender mucho en total y aun así no haber entrado al top 5 del
// año en que se publicó: son dos preguntas distintas.
func TestTopSellingLibroVendedorFueraDelTop5DeSuAño(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	author := mustCreateAuthor(t, s, "Autora")

	// Cinco superventas publicadas en 1990 que arrasan ese año.
	for i := range 5 {
		book := models.Book{AuthorID: author.ID, Name: fmt.Sprintf("Éxito %d", i+1),
			PublicationDate: "1990-01-01"}
		if err := s.Books.Create(ctx, &book); err != nil {
			t.Fatalf("crear libro: %v", err)
		}
		if err := s.Sales.Create(ctx, &models.Sale{BookID: book.ID, Year: 1990, Sales: 10000}); err != nil {
			t.Fatalf("crear venta: %v", err)
		}
	}

	// Un libro también de 1990 que vendió poco ese año pero mucho después.
	slow := models.Book{AuthorID: author.ID, Name: "Lento", PublicationDate: "1990-01-01"}
	if err := s.Books.Create(ctx, &slow); err != nil {
		t.Fatalf("crear libro: %v", err)
	}
	if err := s.Sales.Create(ctx, &models.Sale{BookID: slow.ID, Year: 1990, Sales: 5}); err != nil {
		t.Fatalf("crear venta 1990: %v", err)
	}
	if err := s.Sales.Create(ctx, &models.Sale{BookID: slow.ID, Year: 1995, Sales: 90000}); err != nil {
		t.Fatalf("crear venta 1995: %v", err)
	}

	rows, err := s.Books.TopSelling(ctx, 50)
	if err != nil {
		t.Fatalf("TopSelling: %v", err)
	}

	if rows[0].BookName != "Lento" {
		t.Fatalf("el más vendido es %q, se esperaba Lento (90005)", rows[0].BookName)
	}
	if rows[0].BookSales != 90005 {
		t.Errorf("ventas de Lento = %d, se esperaban 90005", rows[0].BookSales)
	}
	if rows[0].InTop5OfYear {
		t.Error("Lento aparece en el top 5 de 1990, pero ese año vendió 5 contra cinco libros de 10000")
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func almostEqual(a, b float64) bool {
	const epsilon = 1e-9
	diff := a - b
	return diff < epsilon && diff > -epsilon
}

func names(rows []AuthorStatsRow) []string {
	out := make([]string, len(rows))
	for i, row := range rows {
		out[i] = row.Name
	}
	return out
}

func bookNames(rows []TopRatedBookRow) []string {
	out := make([]string, len(rows))
	for i, row := range rows {
		out[i] = row.BookName
	}
	return out
}
