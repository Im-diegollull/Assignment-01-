package store

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"bookreviews/internal/models"
)

func TestSearchTerms(t *testing.T) {
	cases := []struct {
		name  string
		query string
		want  []string
	}{
		{"vacío", "", nil},
		{"solo espacios", "   \t  ", nil},
		{"una palabra", "naufragio", []string{"naufragio"}},
		{"varias palabras", "naufragio memoria", []string{"naufragio", "memoria"}},
		{"pasa a minúscula", "NAUFRAGIO Memoria", []string{"naufragio", "memoria"}},
		{"descarta las de un carácter", "a de la memoria", []string{"de", "la", "memoria"}},
		{"saca la puntuación", "¿naufragio, memoria?", []string{"naufragio", "memoria"}},
		{"conserva acentos", "topógrafa", []string{"topógrafa"}},
		{"quita repetidas", "memoria memoria MEMORIA", []string{"memoria"}},
		{"espacios de más", "  memoria   archivo  ", []string{"memoria", "archivo"}},
		{"conserva números", "1922 archivo", []string{"1922", "archivo"}},
		{"solo puntuación", "??? --- ...", nil},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := SearchTerms(c.query)
			if !slices.Equal(got, c.want) {
				t.Errorf("SearchTerms(%q) = %v, se esperaba %v", c.query, got, c.want)
			}
		})
	}
}

func TestSearchTermsRespetaElTope(t *testing.T) {
	var query string
	for i := range 30 {
		query += fmt.Sprintf("palabra%d ", i)
	}

	if got := SearchTerms(query); len(got) != MaxSearchTerms {
		t.Errorf("devolvió %d términos, se esperaban %d", len(got), MaxSearchTerms)
	}
}

// newSearchFixture crea libros con resúmenes conocidos para poder afirmar
// exactamente qué tiene que traer cada búsqueda.
func newSearchFixture(t *testing.T, s *Store) {
	t.Helper()
	ctx := context.Background()

	author := mustCreateAuthor(t, s, "Autora")

	summaries := map[string]string{
		"Uno":    "Un naufragio frente a la costa y una memoria rota.",
		"Dos":    "La memoria de un pueblo entero, contada desde el archivo.",
		"Tres":   "Un incendio en el archivo municipal.",
		"Cuatro": "Una historia sin ninguna de las palabras buscadas.",
		"Cinco":  "Naufragio, con mayúscula al principio de la frase.",
	}
	for _, name := range []string{"Uno", "Dos", "Tres", "Cuatro", "Cinco"} {
		book := models.Book{
			AuthorID:        author.ID,
			Name:            name,
			Summary:         summaries[name],
			PublicationDate: "2010-01-01",
		}
		if err := s.Books.Create(ctx, &book); err != nil {
			t.Fatalf("crear libro %s: %v", name, err)
		}
	}
}

func TestSearchSemanticaOR(t *testing.T) {
	s := newTestStore(t)
	newSearchFixture(t, s)
	ctx := context.Background()

	cases := []struct {
		name  string
		query string
		want  []string
	}{
		{"una palabra", "naufragio", []string{"Cinco", "Uno"}},
		{"otra palabra", "archivo", []string{"Dos", "Tres"}},
		// Devuelve los que tienen CUALQUIERA de las dos, no los que tienen ambas.
		{"dos palabras: unión", "naufragio archivo",
			[]string{"Cinco", "Dos", "Tres", "Uno"}},
		{"palabra inexistente", "dinosaurio", nil},
		// LIKE de SQLite no distingue mayúsculas en ASCII.
		{"sin distinguir mayúsculas", "NAUFRAGIO", []string{"Cinco", "Uno"}},
		{"input vacío no lista todo", "", nil},
		{"solo puntuación no lista todo", "???", nil},
		// El % es un comodín de SQL: tiene que tratarse como texto literal.
		{"comodín como texto literal", "%", nil},
		{"guión bajo como texto literal", "__", nil},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			books, page, err := s.Books.Search(ctx, SearchTerms(c.query), NewPage(1, 20))
			if err != nil {
				t.Fatalf("Search: %v", err)
			}
			if page.Total != len(c.want) {
				t.Errorf("Total = %d, se esperaban %d", page.Total, len(c.want))
			}

			got := make([]string, len(books))
			for i, book := range books {
				got[i] = book.Name
			}
			if !slices.Equal(got, c.want) {
				t.Errorf("resultados = %v, se esperaban %v", got, c.want)
			}
		})
	}
}

func TestSearchPagina(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	author := mustCreateAuthor(t, s, "Autora")
	for i := range 25 {
		book := models.Book{
			AuthorID:        author.ID,
			Name:            fmt.Sprintf("Libro %02d", i),
			Summary:         "Todos comparten la palabra naufragio.",
			PublicationDate: "2010-01-01",
		}
		if err := s.Books.Create(ctx, &book); err != nil {
			t.Fatalf("crear libro: %v", err)
		}
	}

	terms := SearchTerms("naufragio")

	first, page, err := s.Books.Search(ctx, terms, NewPage(1, 10))
	if err != nil {
		t.Fatalf("Search página 1: %v", err)
	}
	if page.Total != 25 {
		t.Errorf("Total = %d, se esperaban 25", page.Total)
	}
	if page.TotalPages() != 3 {
		t.Errorf("TotalPages = %d, se esperaban 3", page.TotalPages())
	}
	if len(first) != 10 {
		t.Errorf("página 1 trajo %d libros, se esperaban 10", len(first))
	}
	if first[0].Name != "Libro 00" {
		t.Errorf("primer resultado = %q, se esperaba \"Libro 00\"", first[0].Name)
	}

	last, page, err := s.Books.Search(ctx, terms, NewPage(3, 10))
	if err != nil {
		t.Fatalf("Search página 3: %v", err)
	}
	if len(last) != 5 {
		t.Errorf("página 3 trajo %d libros, se esperaban 5", len(last))
	}
	if page.HasNext() {
		t.Error("la última página no debería tener siguiente")
	}
	if last[0].Name != "Libro 20" {
		t.Errorf("primer resultado de la página 3 = %q, se esperaba \"Libro 20\"", last[0].Name)
	}
}

// Una página más allá del último resultado devuelve vacío, pero conservando el
// total: la vista tiene que poder decir "no hay nada acá" sin perder el contexto.
func TestSearchPaginaFueraDeRango(t *testing.T) {
	s := newTestStore(t)
	newSearchFixture(t, s)

	books, page, err := s.Books.Search(context.Background(), SearchTerms("naufragio"), NewPage(99, 20))
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(books) != 0 {
		t.Errorf("devolvió %d libros, se esperaban 0", len(books))
	}
	if page.Total != 2 {
		t.Errorf("Total = %d, se esperaban 2", page.Total)
	}
}

func TestSearchTraeElNombreDelAutor(t *testing.T) {
	s := newTestStore(t)
	newSearchFixture(t, s)

	books, _, err := s.Books.Search(context.Background(), SearchTerms("naufragio"), NewPage(1, 20))
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(books) == 0 {
		t.Fatal("no hubo resultados")
	}
	if books[0].AuthorName != "Autora" {
		t.Errorf("AuthorName = %q, se esperaba \"Autora\"", books[0].AuthorName)
	}
}

func TestEscapeLike(t *testing.T) {
	cases := map[string]string{
		"memoria":  "memoria",
		"100%":     `100\%`,
		"a_b":      `a\_b`,
		`c:\ruta`:  `c:\\ruta`,
		"%_mixto%": `\%\_mixto\%`,
	}
	for input, want := range cases {
		if got := escapeLike(input); got != want {
			t.Errorf("escapeLike(%q) = %q, se esperaba %q", input, got, want)
		}
	}
}
