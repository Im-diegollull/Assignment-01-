package store

import (
	"context"
	"database/sql"
	"fmt"

	"bookreviews/internal/models"
)

type Counts struct {
	Authors int
	Books   int
	Reviews int
	Sales   int
}

func (c Counts) Empty() bool {
	return c.Authors == 0 && c.Books == 0 && c.Reviews == 0 && c.Sales == 0
}

func (s *Store) Count(ctx context.Context) (Counts, error) {
	const query = `
SELECT (SELECT COUNT(*) FROM authors),
       (SELECT COUNT(*) FROM books),
       (SELECT COUNT(*) FROM reviews),
       (SELECT COUNT(*) FROM sales_by_year)`

	var c Counts
	if err := s.db.QueryRowContext(ctx, query).Scan(&c.Authors, &c.Books, &c.Reviews, &c.Sales); err != nil {
		return c, fmt.Errorf("store: contar filas: %w", err)
	}
	return c, nil
}

// Reset vacía las cuatro tablas. También limpia sqlite_sequence, que es donde
// AUTOINCREMENT guarda el último id usado: sin eso, volver a sembrar con la
// misma semilla generaría los mismos datos pero con ids corridos.
func (s *Store) Reset(ctx context.Context) error {
	return inTx(ctx, s.db, func(tx *sql.Tx) error {
		// El orden importa aunque haya ON DELETE CASCADE: borrar de la hoja a
		// la raíz evita depender de él.
		tables := []string{"sales_by_year", "reviews", "books", "authors"}
		for _, table := range tables {
			if _, err := tx.ExecContext(ctx, `DELETE FROM `+table); err != nil {
				return fmt.Errorf("store: vaciar %s: %w", table, err)
			}
		}

		const resetSeq = `DELETE FROM sqlite_sequence
WHERE name IN ('authors', 'books', 'reviews', 'sales_by_year')`
		if _, err := tx.ExecContext(ctx, resetSeq); err != nil {
			return fmt.Errorf("store: reiniciar sqlite_sequence: %w", err)
		}
		return nil
	})
}

// CreateManyAuthors inserta los autores y completa el ID de cada uno.
func (s *AuthorStore) CreateMany(ctx context.Context, authors []models.Author) error {
	return inTx(ctx, s.db, func(tx *sql.Tx) error {
		stmt, err := tx.PrepareContext(ctx, createAuthorSQL)
		if err != nil {
			return fmt.Errorf("store: preparar alta de autores: %w", err)
		}
		defer stmt.Close()

		for i := range authors {
			res, err := stmt.ExecContext(ctx, authors[i].Name, authors[i].DateOfBirth,
				authors[i].CountryOfOrigin, authors[i].Description)
			if err != nil {
				return fmt.Errorf("store: insertar autor %q: %w", authors[i].Name, err)
			}
			id, err := res.LastInsertId()
			if err != nil {
				return fmt.Errorf("store: insertar autor %q: leer id: %w", authors[i].Name, err)
			}
			authors[i].ID = id
		}
		return nil
	})
}

func (s *BookStore) CreateMany(ctx context.Context, books []models.Book) error {
	return inTx(ctx, s.db, func(tx *sql.Tx) error {
		stmt, err := tx.PrepareContext(ctx, createBookSQL)
		if err != nil {
			return fmt.Errorf("store: preparar alta de libros: %w", err)
		}
		defer stmt.Close()

		for i := range books {
			res, err := stmt.ExecContext(ctx, books[i].AuthorID, books[i].Name,
				books[i].Summary, books[i].PublicationDate, books[i].NumberOfSales)
			if err != nil {
				return fmt.Errorf("store: insertar libro %q: %w", books[i].Name, err)
			}
			id, err := res.LastInsertId()
			if err != nil {
				return fmt.Errorf("store: insertar libro %q: leer id: %w", books[i].Name, err)
			}
			books[i].ID = id
		}
		return nil
	})
}

func (s *ReviewStore) CreateMany(ctx context.Context, reviews []models.Review) error {
	return inTx(ctx, s.db, func(tx *sql.Tx) error {
		stmt, err := tx.PrepareContext(ctx, createReviewSQL)
		if err != nil {
			return fmt.Errorf("store: preparar alta de reseñas: %w", err)
		}
		defer stmt.Close()

		for i := range reviews {
			if _, err := stmt.ExecContext(ctx, reviews[i].BookID, reviews[i].Text,
				reviews[i].Score, reviews[i].Upvotes); err != nil {
				return fmt.Errorf("store: insertar reseña del libro %d: %w", reviews[i].BookID, err)
			}
		}
		return nil
	})
}

// CreateMany inserta las ventas y recalcula de una sola vez el total de todos
// los libros, en la misma transacción. No usa recalcBookSales por libro: acá se
// insertan miles de filas y una única sentencia sobre toda la tabla es mucho
// más barata que una por libro.
func (s *SaleStore) CreateMany(ctx context.Context, sales []models.Sale) error {
	return inTx(ctx, s.db, func(tx *sql.Tx) error {
		stmt, err := tx.PrepareContext(ctx, createSaleSQL)
		if err != nil {
			return fmt.Errorf("store: preparar alta de ventas: %w", err)
		}
		defer stmt.Close()

		for i := range sales {
			if _, err := stmt.ExecContext(ctx, sales[i].BookID, sales[i].Year,
				sales[i].Sales); err != nil {
				return fmt.Errorf("store: insertar ventas %d del libro %d: %w",
					sales[i].Year, sales[i].BookID, err)
			}
		}
		return recalcAllBookSales(ctx, tx)
	})
}

const recalcAllBookSalesSQL = `
UPDATE books
SET number_of_sales = COALESCE(
  (SELECT SUM(s.sales) FROM sales_by_year s WHERE s.book_id = books.id), 0)`

// RecalculateAllBookSales deja books.number_of_sales consistente con
// sales_by_year para todos los libros de una sola pasada.
func (s *SaleStore) RecalculateAllBookSales(ctx context.Context) error {
	return recalcAllBookSales(ctx, s.db)
}

func recalcAllBookSales(ctx context.Context, exec executor) error {
	if _, err := exec.ExecContext(ctx, recalcAllBookSalesSQL); err != nil {
		return fmt.Errorf("store: recalcular las ventas de todos los libros: %w", err)
	}
	return nil
}
