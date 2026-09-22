package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"bookreviews/internal/models"
)

type AuthorStore struct{ db *sql.DB }

// Las columnas opcionales salen con COALESCE para que un NULL se lea como ""
// y models.Author pueda usar string en vez de sql.NullString.
const authorColumns = `id, name, COALESCE(date_of_birth, ''), ` +
	`COALESCE(country_of_origin, ''), COALESCE(description, ''), ` +
	`COALESCE((SELECT path FROM author_images WHERE author_id = authors.id), '')`

const listAuthorsSQL = `
SELECT ` + authorColumns + `
FROM authors
ORDER BY name COLLATE NOCASE
LIMIT ? OFFSET ?`

func (s *AuthorStore) List(ctx context.Context, page Page) ([]models.Author, Page, error) {
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM authors`).Scan(&page.Total); err != nil {
		return nil, page, fmt.Errorf("store: contar autores: %w", err)
	}

	rows, err := s.db.QueryContext(ctx, listAuthorsSQL, page.Size, page.Offset())
	if err != nil {
		return nil, page, fmt.Errorf("store: listar autores: %w", err)
	}
	defer rows.Close()

	authors := make([]models.Author, 0, page.Size)
	for rows.Next() {
		var a models.Author
		if err := scanAuthor(rows, &a); err != nil {
			return nil, page, fmt.Errorf("store: listar autores: %w", err)
		}
		authors = append(authors, a)
	}
	if err := rows.Err(); err != nil {
		return nil, page, fmt.Errorf("store: listar autores: %w", err)
	}
	return authors, page, nil
}

const allAuthorsSQL = `SELECT id, name FROM authors ORDER BY name COLLATE NOCASE`

func (s *AuthorStore) All(ctx context.Context) ([]models.Author, error) {
	rows, err := s.db.QueryContext(ctx, allAuthorsSQL)
	if err != nil {
		return nil, fmt.Errorf("store: listar todos los autores: %w", err)
	}
	defer rows.Close()

	var authors []models.Author
	for rows.Next() {
		var a models.Author
		if err := rows.Scan(&a.ID, &a.Name); err != nil {
			return nil, fmt.Errorf("store: listar todos los autores: %w", err)
		}
		authors = append(authors, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: listar todos los autores: %w", err)
	}
	return authors, nil
}

const getAuthorSQL = `SELECT ` + authorColumns + ` FROM authors WHERE id = ?`

func (s *AuthorStore) Get(ctx context.Context, id int64) (models.Author, error) {
	var a models.Author
	err := scanAuthor(s.db.QueryRowContext(ctx, getAuthorSQL, id), &a)
	if errors.Is(err, sql.ErrNoRows) {
		return a, ErrNotFound
	}
	if err != nil {
		return a, fmt.Errorf("store: obtener autor %d: %w", id, err)
	}
	return a, nil
}

const createAuthorSQL = `
INSERT INTO authors (name, date_of_birth, country_of_origin, description)
VALUES (?, ?, ?, ?)`

func (s *AuthorStore) Create(ctx context.Context, a *models.Author) error {
	res, err := s.db.ExecContext(ctx, createAuthorSQL,
		a.Name, a.DateOfBirth, a.CountryOfOrigin, a.Description)
	if err != nil {
		return fmt.Errorf("store: crear autor: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("store: crear autor: leer id: %w", err)
	}
	a.ID = id
	return nil
}

const updateAuthorSQL = `
UPDATE authors
SET name = ?, date_of_birth = ?, country_of_origin = ?, description = ?
WHERE id = ?`

func (s *AuthorStore) Update(ctx context.Context, a *models.Author) error {
	res, err := s.db.ExecContext(ctx, updateAuthorSQL,
		a.Name, a.DateOfBirth, a.CountryOfOrigin, a.Description, a.ID)
	if err != nil {
		return fmt.Errorf("store: actualizar autor %d: %w", a.ID, err)
	}
	return affectedOne(res, fmt.Sprintf("actualizar autor %d", a.ID))
}

func (s *AuthorStore) Delete(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM authors WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("store: borrar autor %d: %w", id, err)
	}
	return affectedOne(res, fmt.Sprintf("borrar autor %d", id))
}

type scanner interface{ Scan(dest ...any) error }

func scanAuthor(src scanner, a *models.Author) error {
	return src.Scan(&a.ID, &a.Name, &a.DateOfBirth, &a.CountryOfOrigin, &a.Description, &a.ImagePath)
}
