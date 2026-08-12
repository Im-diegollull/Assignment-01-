package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

var ErrNotFound = errors.New("store: registro no encontrado")

type Store struct {
	Authors *AuthorStore
	Books   *BookStore
	Reviews *ReviewStore
	Sales   *SaleStore

	db *sql.DB
}

func New(db *sql.DB) *Store {
	return &Store{
		Authors: &AuthorStore{db: db},
		Books:   &BookStore{db: db},
		Reviews: &ReviewStore{db: db},
		Sales:   &SaleStore{db: db},
		db:      db,
	}
}

type executor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func inTx(ctx context.Context, db *sql.DB, fn func(tx *sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: iniciar transacción: %w", err)
	}

	if err := fn(tx); err != nil {

		_ = tx.Rollback()
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: commit: %w", err)
	}
	return nil
}

func affectedOne(res sql.Result, operation string) error {
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("store: %s: leer filas afectadas: %w", operation, err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}
