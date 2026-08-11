package store

import (
	"context"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	// MaxSearchTerms acota el tamaño del OR que se arma con los términos. Sin
	// tope, una consulta con 500 palabras generaría 500 LIKE con comodín a
	// ambos lados, cada uno un scan completo de la tabla.
	MaxSearchTerms = 10

	// minTermRunes descarta las palabras de un solo carácter: con un comodín a
	// cada lado, "a" matchea prácticamente todos los resúmenes y volvería
	// inútil la semántica de "cualquiera de las palabras".
	minTermRunes = 2
)

// SearchTerms convierte el texto libre en la lista de palabras a buscar.
//
// Es una función pura, sin base de datos, así que se puede testear sola. El
// handler la usa además para distinguir "el usuario no buscó nada" de "buscó y
// no hubo resultados".
func SearchTerms(query string) []string {
	terms := make([]string, 0, MaxSearchTerms)
	seen := make(map[string]bool, MaxSearchTerms)

	for _, field := range strings.Fields(strings.ToLower(query)) {
		// La puntuación pegada a la palabra ensuciaría el LIKE: buscar
		// "naufragio," no encontraría "naufragio" seguido de otra cosa.
		word := strings.TrimFunc(field, func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsDigit(r)
		})

		if utf8.RuneCountInString(word) < minTermRunes || seen[word] {
			continue
		}
		seen[word] = true

		terms = append(terms, word)
		if len(terms) == MaxSearchTerms {
			break
		}
	}
	return terms
}

// Search devuelve la página de libros cuyo resumen contenga **alguna** de las
// palabras (semántica OR, que es lo que pide el enunciado).
//
// Sin términos devuelve vacío sin tocar la base: listar los 300 libros porque
// el input estaba en blanco sería peor que no responder nada.
func (s *BookStore) Search(ctx context.Context, terms []string, page Page) ([]BookWithAuthor, Page, error) {
	page.Total = 0
	if len(terms) == 0 {
		return nil, page, nil
	}

	// Un LIKE por término, unidos con OR. Los placeholders se generan según la
	// cantidad de términos; los valores viajan siempre como argumentos.
	conditions := make([]string, len(terms))
	args := make([]any, len(terms))
	for i, term := range terms {
		conditions[i] = `b.summary LIKE ? ESCAPE '\'`
		args[i] = "%" + escapeLike(term) + "%"
	}
	where := "\nWHERE " + strings.Join(conditions, "\n   OR ")

	// Dos consultas: una cuenta el total para saber cuántas páginas hay, la
	// otra trae solo la página pedida.
	countQuery := "SELECT COUNT(*) FROM books b" + where
	if err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&page.Total); err != nil {
		return nil, page, fmt.Errorf("store: contar resultados de búsqueda: %w", err)
	}
	if page.Total == 0 {
		return nil, page, nil
	}

	pageQuery := `
SELECT ` + bookColumns + `, a.name
FROM books b
JOIN authors a ON a.id = b.author_id` + where + `
ORDER BY b.name COLLATE NOCASE, b.id
LIMIT ? OFFSET ?`

	pageArgs := append(append([]any(nil), args...), page.Size, page.Offset())
	rows, err := s.db.QueryContext(ctx, pageQuery, pageArgs...)
	if err != nil {
		return nil, page, fmt.Errorf("store: buscar libros: %w", err)
	}
	defer rows.Close()

	books, err := scanBooksWithAuthor(rows)
	if err != nil {
		return nil, page, fmt.Errorf("store: buscar libros: %w", err)
	}
	return books, page, nil
}

// escapeLike neutraliza los comodines de LIKE en el texto del usuario. Sin
// esto, buscar "%" traería los 300 libros y "_" cualquier carácter suelto: son
// comodines de SQL, no texto literal. Va de la mano con la cláusula
// ESCAPE '\' de la consulta.
func escapeLike(term string) string {
	return likeEscaper.Replace(term)
}

// El backslash se reemplaza primero para no volver a escapar los que agrega el
// propio Replacer; strings.Replacer ya recorre el texto una sola vez, así que
// el orden de las reglas alcanza.
var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
