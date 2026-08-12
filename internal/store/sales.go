package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"bookreviews/internal/models"
)

type SaleStore struct{ db *sql.DB }

var ErrDuplicateSaleYear = errors.New("store: el libro ya tiene ventas registradas para ese año")

type SaleWithBook struct {
	models.Sale
	BookName string
}

const listSalesSQL = `
SELECT s.id, s.book_id, s.year, s.sales, b.name
FROM sales_by_year s
JOIN books b ON b.id = s.book_id
ORDER BY s.year DESC, b.name COLLATE NOCASE
LIMIT ? OFFSET ?`

func (s *SaleStore) List(ctx context.Context, page Page) ([]SaleWithBook, Page, error) {
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sales_by_year`).Scan(&page.Total); err != nil {
		return nil, page, fmt.Errorf("store: contar ventas: %w", err)
	}

	rows, err := s.db.QueryContext(ctx, listSalesSQL, page.Size, page.Offset())
	if err != nil {
		return nil, page, fmt.Errorf("store: listar ventas: %w", err)
	}
	defer rows.Close()

	sales, err := scanSalesWithBook(rows)
	if err != nil {
		return nil, page, fmt.Errorf("store: listar ventas: %w", err)
	}
	return sales, page, nil
}

const listSalesByBookSQL = `
SELECT s.id, s.book_id, s.year, s.sales, b.name
FROM sales_by_year s
JOIN books b ON b.id = s.book_id
WHERE s.book_id = ?
ORDER BY s.year`

func (s *SaleStore) ListByBook(ctx context.Context, bookID int64) ([]SaleWithBook, error) {
	rows, err := s.db.QueryContext(ctx, listSalesByBookSQL, bookID)
	if err != nil {
		return nil, fmt.Errorf("store: listar ventas del libro %d: %w", bookID, err)
	}
	defer rows.Close()

	sales, err := scanSalesWithBook(rows)
	if err != nil {
		return nil, fmt.Errorf("store: listar ventas del libro %d: %w", bookID, err)
	}
	return sales, nil
}

const getSaleSQL = `
SELECT s.id, s.book_id, s.year, s.sales, b.name
FROM sales_by_year s
JOIN books b ON b.id = s.book_id
WHERE s.id = ?`

func (s *SaleStore) Get(ctx context.Context, id int64) (SaleWithBook, error) {
	var sale SaleWithBook
	err := s.db.QueryRowContext(ctx, getSaleSQL, id).Scan(
		&sale.ID, &sale.BookID, &sale.Year, &sale.Sales, &sale.BookName)
	if errors.Is(err, sql.ErrNoRows) {
		return sale, ErrNotFound
	}
	if err != nil {
		return sale, fmt.Errorf("store: obtener venta %d: %w", id, err)
	}
	return sale, nil
}

const createSaleSQL = `INSERT INTO sales_by_year (book_id, year, sales) VALUES (?, ?, ?)`

func (s *SaleStore) Create(ctx context.Context, sale *models.Sale) error {
	return inTx(ctx, s.db, func(tx *sql.Tx) error {
		taken, err := saleYearTaken(ctx, tx, sale.BookID, sale.Year, 0)
		if err != nil {
			return err
		}
		if taken {
			return ErrDuplicateSaleYear
		}

		res, err := tx.ExecContext(ctx, createSaleSQL, sale.BookID, sale.Year, sale.Sales)
		if err != nil {
			return fmt.Errorf("store: crear venta: %w", err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return fmt.Errorf("store: crear venta: leer id: %w", err)
		}
		sale.ID = id

		return recalcBookSales(ctx, tx, sale.BookID)
	})
}

const updateSaleSQL = `UPDATE sales_by_year SET book_id = ?, year = ?, sales = ? WHERE id = ?`

func (s *SaleStore) Update(ctx context.Context, sale *models.Sale) error {
	return inTx(ctx, s.db, func(tx *sql.Tx) error {
		var previousBookID int64
		err := tx.QueryRowContext(ctx,
			`SELECT book_id FROM sales_by_year WHERE id = ?`, sale.ID).Scan(&previousBookID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("store: actualizar venta %d: leer libro anterior: %w", sale.ID, err)
		}

		taken, err := saleYearTaken(ctx, tx, sale.BookID, sale.Year, sale.ID)
		if err != nil {
			return err
		}
		if taken {
			return ErrDuplicateSaleYear
		}

		res, err := tx.ExecContext(ctx, updateSaleSQL,
			sale.BookID, sale.Year, sale.Sales, sale.ID)
		if err != nil {
			return fmt.Errorf("store: actualizar venta %d: %w", sale.ID, err)
		}
		if err := affectedOne(res, fmt.Sprintf("actualizar venta %d", sale.ID)); err != nil {
			return err
		}

		if err := recalcBookSales(ctx, tx, sale.BookID); err != nil {
			return err
		}
		if previousBookID != sale.BookID {
			return recalcBookSales(ctx, tx, previousBookID)
		}
		return nil
	})
}

func (s *SaleStore) Delete(ctx context.Context, id int64) error {
	return inTx(ctx, s.db, func(tx *sql.Tx) error {
		var bookID int64
		err := tx.QueryRowContext(ctx,
			`SELECT book_id FROM sales_by_year WHERE id = ?`, id).Scan(&bookID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("store: borrar venta %d: leer libro: %w", id, err)
		}

		if _, err := tx.ExecContext(ctx, `DELETE FROM sales_by_year WHERE id = ?`, id); err != nil {
			return fmt.Errorf("store: borrar venta %d: %w", id, err)
		}
		return recalcBookSales(ctx, tx, bookID)
	})
}

const recalcBookSalesSQL = `
UPDATE books
SET number_of_sales = COALESCE((SELECT SUM(sales) FROM sales_by_year WHERE book_id = ?), 0)
WHERE id = ?`

func recalcBookSales(ctx context.Context, exec executor, bookID int64) error {
	if _, err := exec.ExecContext(ctx, recalcBookSalesSQL, bookID, bookID); err != nil {
		return fmt.Errorf("store: recalcular ventas del libro %d: %w", bookID, err)
	}
	return nil
}

func saleYearTaken(ctx context.Context, exec executor, bookID int64, year int, excludeID int64) (bool, error) {
	const query = `SELECT 1 FROM sales_by_year WHERE book_id = ? AND year = ? AND id <> ? LIMIT 1`

	var found int
	err := exec.QueryRowContext(ctx, query, bookID, year, excludeID).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("store: verificar ventas duplicadas: %w", err)
	}
	return true, nil
}

func scanSalesWithBook(rows *sql.Rows) ([]SaleWithBook, error) {
	var sales []SaleWithBook
	for rows.Next() {
		var sale SaleWithBook
		if err := rows.Scan(&sale.ID, &sale.BookID, &sale.Year,
			&sale.Sales, &sale.BookName); err != nil {
			return nil, err
		}
		sales = append(sales, sale)
	}
	return sales, rows.Err()
}
