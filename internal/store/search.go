package store

import (
	"context"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	MaxSearchTerms = 10

	minTermRunes = 2
)

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

func (s *BookStore) Search(ctx context.Context, terms []string, page Page) ([]BookWithAuthor, Page, error) {
	page.Total = 0
	if len(terms) == 0 {
		return nil, page, nil
	}

	conditions := make([]string, len(terms))
	args := make([]any, len(terms))
	for i, term := range terms {
		conditions[i] = `b.summary LIKE ? ESCAPE '\'`
		args[i] = "%" + escapeLike(term) + "%"
	}
	where := "\nWHERE " + strings.Join(conditions, "\n   OR ")

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

func escapeLike(term string) string {
	return likeEscaper.Replace(term)
}

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
