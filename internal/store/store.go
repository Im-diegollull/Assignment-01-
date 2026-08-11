// Package store es la capa de acceso a datos: acá vive todo el SQL del
// proyecto. No conoce net/http ni html/template.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// ErrNotFound se devuelve cuando una fila buscada por id no existe. El store no
// sabe nada de códigos HTTP: la traducción a 404 ocurre solo en handlers.
var ErrNotFound = errors.New("store: registro no encontrado")

// Store agrupa un sub-store por agregado. Se construye una vez en main.
type Store struct {
	Authors *AuthorStore
	Books   *BookStore
	Reviews *ReviewStore
	Sales   *SaleStore
}

func New(db *sql.DB) *Store {
	return &Store{
		Authors: &AuthorStore{db: db},
		Books:   &BookStore{db: db},
		Reviews: &ReviewStore{db: db},
		Sales:   &SaleStore{db: db},
	}
}

// executor abstrae lo común entre *sql.DB y *sql.Tx, para que una función que
// escribe pueda usarse suelta o dentro de una transacción sin duplicar código.
type executor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// inTx corre fn dentro de una transacción y hace rollback si devuelve error.
// Es el Unit of Work de las operaciones que tocan más de una tabla (crear una
// venta y recalcular books.number_of_sales tienen que pasar o fallar juntas).
func inTx(ctx context.Context, db *sql.DB, fn func(tx *sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: iniciar transacción: %w", err)
	}

	if err := fn(tx); err != nil {
		// El error del rollback se ignora a propósito: el que importa es el que
		// causó el fallo, y devolverlo tapado sería peor.
		_ = tx.Rollback()
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: commit: %w", err)
	}
	return nil
}

// affectedOne traduce "0 filas afectadas" a ErrNotFound, para que UPDATE y
// DELETE sobre un id inexistente se comporten igual que un Get fallido.
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
