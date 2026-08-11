package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"bookreviews/internal/models"
)

type ReviewStore struct{ db *sql.DB }

// ReviewWithBook es el view model de los listados de reseñas: agrega el nombre
// del libro reseñado, que viene de un JOIN.
type ReviewWithBook struct {
	models.Review
	BookName string
}

const reviewColumns = `r.id, r.book_id, COALESCE(r.review, ''), r.score, r.upvotes`

const listReviewsSQL = `
SELECT ` + reviewColumns + `, b.name
FROM reviews r
JOIN books b ON b.id = r.book_id
ORDER BY r.id DESC
LIMIT ? OFFSET ?`

func (s *ReviewStore) List(ctx context.Context, page Page) ([]ReviewWithBook, Page, error) {
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM reviews`).Scan(&page.Total); err != nil {
		return nil, page, fmt.Errorf("store: contar reseñas: %w", err)
	}

	rows, err := s.db.QueryContext(ctx, listReviewsSQL, page.Size, page.Offset())
	if err != nil {
		return nil, page, fmt.Errorf("store: listar reseñas: %w", err)
	}
	defer rows.Close()

	reviews, err := scanReviewsWithBook(rows)
	if err != nil {
		return nil, page, fmt.Errorf("store: listar reseñas: %w", err)
	}
	return reviews, page, nil
}

const listReviewsByBookSQL = `
SELECT ` + reviewColumns + `, b.name
FROM reviews r
JOIN books b ON b.id = r.book_id
WHERE r.book_id = ?
ORDER BY r.score DESC, r.upvotes DESC`

// ListByBook alimenta la ficha del libro: como máximo 10 reseñas por libro.
func (s *ReviewStore) ListByBook(ctx context.Context, bookID int64) ([]ReviewWithBook, error) {
	rows, err := s.db.QueryContext(ctx, listReviewsByBookSQL, bookID)
	if err != nil {
		return nil, fmt.Errorf("store: listar reseñas del libro %d: %w", bookID, err)
	}
	defer rows.Close()

	reviews, err := scanReviewsWithBook(rows)
	if err != nil {
		return nil, fmt.Errorf("store: listar reseñas del libro %d: %w", bookID, err)
	}
	return reviews, nil
}

const getReviewSQL = `
SELECT ` + reviewColumns + `, b.name
FROM reviews r
JOIN books b ON b.id = r.book_id
WHERE r.id = ?`

func (s *ReviewStore) Get(ctx context.Context, id int64) (ReviewWithBook, error) {
	var r ReviewWithBook
	err := s.db.QueryRowContext(ctx, getReviewSQL, id).Scan(
		&r.ID, &r.BookID, &r.Text, &r.Score, &r.Upvotes, &r.BookName)
	if errors.Is(err, sql.ErrNoRows) {
		return r, ErrNotFound
	}
	if err != nil {
		return r, fmt.Errorf("store: obtener reseña %d: %w", id, err)
	}
	return r, nil
}

const createReviewSQL = `
INSERT INTO reviews (book_id, review, score, upvotes) VALUES (?, ?, ?, ?)`

func (s *ReviewStore) Create(ctx context.Context, r *models.Review) error {
	res, err := s.db.ExecContext(ctx, createReviewSQL, r.BookID, r.Text, r.Score, r.Upvotes)
	if err != nil {
		return fmt.Errorf("store: crear reseña: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("store: crear reseña: leer id: %w", err)
	}
	r.ID = id
	return nil
}

const updateReviewSQL = `
UPDATE reviews SET book_id = ?, review = ?, score = ?, upvotes = ? WHERE id = ?`

func (s *ReviewStore) Update(ctx context.Context, r *models.Review) error {
	res, err := s.db.ExecContext(ctx, updateReviewSQL,
		r.BookID, r.Text, r.Score, r.Upvotes, r.ID)
	if err != nil {
		return fmt.Errorf("store: actualizar reseña %d: %w", r.ID, err)
	}
	return affectedOne(res, fmt.Sprintf("actualizar reseña %d", r.ID))
}

func (s *ReviewStore) Delete(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM reviews WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("store: borrar reseña %d: %w", id, err)
	}
	return affectedOne(res, fmt.Sprintf("borrar reseña %d", id))
}

func scanReviewsWithBook(rows *sql.Rows) ([]ReviewWithBook, error) {
	var reviews []ReviewWithBook
	for rows.Next() {
		var r ReviewWithBook
		if err := rows.Scan(&r.ID, &r.BookID, &r.Text, &r.Score,
			&r.Upvotes, &r.BookName); err != nil {
			return nil, err
		}
		reviews = append(reviews, r)
	}
	return reviews, rows.Err()
}
