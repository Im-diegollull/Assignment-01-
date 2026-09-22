package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"bookreviews/internal/models"
)

type BookStore struct{ db *sql.DB }

type BookWithAuthor struct {
	models.Book
	AuthorName string
}

const bookColumns = `b.id, b.author_id, b.name, COALESCE(b.summary, ''), ` +
	`COALESCE(b.publication_date, ''), b.number_of_sales, ` +
	`COALESCE((SELECT path FROM book_images WHERE book_id = b.id), '')`

const listBooksSQL = `
SELECT ` + bookColumns + `, a.name
FROM books b
JOIN authors a ON a.id = b.author_id
ORDER BY b.name COLLATE NOCASE
LIMIT ? OFFSET ?`

func (s *BookStore) List(ctx context.Context, page Page) ([]BookWithAuthor, Page, error) {
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM books`).Scan(&page.Total); err != nil {
		return nil, page, fmt.Errorf("store: contar libros: %w", err)
	}

	rows, err := s.db.QueryContext(ctx, listBooksSQL, page.Size, page.Offset())
	if err != nil {
		return nil, page, fmt.Errorf("store: listar libros: %w", err)
	}
	defer rows.Close()

	books, err := scanBooksWithAuthor(rows)
	if err != nil {
		return nil, page, fmt.Errorf("store: listar libros: %w", err)
	}
	return books, page, nil
}

const listBooksByAuthorSQL = `
SELECT ` + bookColumns + `, a.name
FROM books b
JOIN authors a ON a.id = b.author_id
WHERE b.author_id = ?
ORDER BY b.publication_date DESC`

func (s *BookStore) ListByAuthor(ctx context.Context, authorID int64) ([]BookWithAuthor, error) {
	rows, err := s.db.QueryContext(ctx, listBooksByAuthorSQL, authorID)
	if err != nil {
		return nil, fmt.Errorf("store: listar libros del autor %d: %w", authorID, err)
	}
	defer rows.Close()

	books, err := scanBooksWithAuthor(rows)
	if err != nil {
		return nil, fmt.Errorf("store: listar libros del autor %d: %w", authorID, err)
	}
	return books, nil
}

const allBooksSQL = `
SELECT id, name, COALESCE(publication_date, '')
FROM books
ORDER BY name COLLATE NOCASE`

// All alimenta los <select> de reseñas y ventas.
func (s *BookStore) All(ctx context.Context) ([]models.Book, error) {
	rows, err := s.db.QueryContext(ctx, allBooksSQL)
	if err != nil {
		return nil, fmt.Errorf("store: listar todos los libros: %w", err)
	}
	defer rows.Close()

	var books []models.Book
	for rows.Next() {
		var b models.Book
		if err := rows.Scan(&b.ID, &b.Name, &b.PublicationDate); err != nil {
			return nil, fmt.Errorf("store: listar todos los libros: %w", err)
		}
		books = append(books, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: listar todos los libros: %w", err)
	}
	return books, nil
}

const getBookSQL = `
SELECT ` + bookColumns + `, a.name
FROM books b
JOIN authors a ON a.id = b.author_id
WHERE b.id = ?`

func (s *BookStore) Get(ctx context.Context, id int64) (BookWithAuthor, error) {
	var b BookWithAuthor
	err := s.db.QueryRowContext(ctx, getBookSQL, id).Scan(
		&b.ID, &b.AuthorID, &b.Name, &b.Summary, &b.PublicationDate,
		&b.NumberOfSales, &b.ImagePath, &b.AuthorName)
	if errors.Is(err, sql.ErrNoRows) {
		return b, ErrNotFound
	}
	if err != nil {
		return b, fmt.Errorf("store: obtener libro %d: %w", id, err)
	}
	return b, nil
}

const createBookSQL = `
INSERT INTO books (author_id, name, summary, publication_date, number_of_sales)
VALUES (?, ?, ?, ?, ?)`

func (s *BookStore) Create(ctx context.Context, b *models.Book) error {
	res, err := s.db.ExecContext(ctx, createBookSQL,
		b.AuthorID, b.Name, b.Summary, b.PublicationDate, b.NumberOfSales)
	if err != nil {
		return fmt.Errorf("store: crear libro: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("store: crear libro: leer id: %w", err)
	}
	b.ID = id
	return nil
}

const updateBookSQL = `
UPDATE books
SET author_id = ?, name = ?, summary = ?, publication_date = ?, number_of_sales = ?
WHERE id = ?`

func (s *BookStore) Update(ctx context.Context, b *models.Book) error {
	res, err := s.db.ExecContext(ctx, updateBookSQL,
		b.AuthorID, b.Name, b.Summary, b.PublicationDate, b.NumberOfSales, b.ID)
	if err != nil {
		return fmt.Errorf("store: actualizar libro %d: %w", b.ID, err)
	}
	return affectedOne(res, fmt.Sprintf("actualizar libro %d", b.ID))
}

func (s *BookStore) Delete(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM books WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("store: borrar libro %d: %w", id, err)
	}
	return affectedOne(res, fmt.Sprintf("borrar libro %d", id))
}

func scanBooksWithAuthor(rows *sql.Rows) ([]BookWithAuthor, error) {
	var books []BookWithAuthor
	for rows.Next() {
		var b BookWithAuthor
		if err := rows.Scan(&b.ID, &b.AuthorID, &b.Name, &b.Summary,
			&b.PublicationDate, &b.NumberOfSales, &b.ImagePath, &b.AuthorName); err != nil {
			return nil, err
		}
		books = append(books, b)
	}
	return books, rows.Err()
}
