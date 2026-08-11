package database

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

// newTestDB abre una base temporal ya migrada. t.TempDir se limpia solo al
// terminar el test.
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := Migrate(context.Background(), db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return db
}

func TestOpenAplicaLosPragmas(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	var foreignKeys int
	if err := db.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		t.Fatalf("leer foreign_keys: %v", err)
	}
	if foreignKeys != 1 {
		t.Errorf("foreign_keys = %d, se esperaba 1", foreignKeys)
	}

	var journalMode string
	if err := db.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatalf("leer journal_mode: %v", err)
	}
	if journalMode != "wal" {
		t.Errorf("journal_mode = %q, se esperaba \"wal\"", journalMode)
	}
}

func TestMigrateEsIdempotente(t *testing.T) {
	db := newTestDB(t)

	// newTestDB ya migró una vez; la segunda pasada no debe fallar.
	if err := Migrate(context.Background(), db); err != nil {
		t.Fatalf("segunda Migrate: %v", err)
	}
}

// Esta es la verificación central de la fase: sin el pragma foreign_keys, SQLite
// acepta en silencio un libro cuyo autor no existe.
func TestForeignKeysRechazanReferenciasInexistentes(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	_, err := db.ExecContext(ctx,
		`INSERT INTO books (author_id, name) VALUES (?, ?)`, 9999, "Libro huérfano")
	if err == nil {
		t.Fatal("se insertó un libro con author_id inexistente: las FK no se están aplicando")
	}

	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM books`).Scan(&count); err != nil {
		t.Fatalf("contar libros: %v", err)
	}
	if count != 0 {
		t.Errorf("books tiene %d filas, se esperaba 0", count)
	}
}

func TestOnDeleteCascadeBorraLosHijos(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	authorID := insertAuthor(t, db, "Autora de prueba")
	bookID := insertBook(t, db, authorID, "Libro de prueba")

	if _, err := db.ExecContext(ctx,
		`INSERT INTO reviews (book_id, review, score) VALUES (?, ?, ?)`,
		bookID, "Buena", 4); err != nil {
		t.Fatalf("insertar reseña: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO sales_by_year (book_id, year, sales) VALUES (?, ?, ?)`,
		bookID, 2020, 100); err != nil {
		t.Fatalf("insertar venta: %v", err)
	}

	// Borrar el autor debe arrastrar libros → reseñas y ventas, en cascada.
	if _, err := db.ExecContext(ctx, `DELETE FROM authors WHERE id = ?`, authorID); err != nil {
		t.Fatalf("borrar autor: %v", err)
	}

	for _, table := range []string{"books", "reviews", "sales_by_year"} {
		var count int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table).Scan(&count); err != nil {
			t.Fatalf("contar %s: %v", table, err)
		}
		if count != 0 {
			t.Errorf("%s quedó con %d filas tras borrar el autor, se esperaba 0", table, count)
		}
	}
}

func TestScoreFueraDeRangoEsRechazado(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	bookID := insertBook(t, db, insertAuthor(t, db, "Autora"), "Libro")

	for _, score := range []int{0, 6, -1} {
		_, err := db.ExecContext(ctx,
			`INSERT INTO reviews (book_id, review, score) VALUES (?, ?, ?)`,
			bookID, "texto", score)
		if err == nil {
			t.Errorf("se aceptó una reseña con score = %d; el CHECK no se aplicó", score)
		}
	}
}

func TestUnaSolaFilaDeVentasPorLibroYAño(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	bookID := insertBook(t, db, insertAuthor(t, db, "Autora"), "Libro")

	if _, err := db.ExecContext(ctx,
		`INSERT INTO sales_by_year (book_id, year, sales) VALUES (?, ?, ?)`,
		bookID, 2021, 10); err != nil {
		t.Fatalf("primera venta: %v", err)
	}

	_, err := db.ExecContext(ctx,
		`INSERT INTO sales_by_year (book_id, year, sales) VALUES (?, ?, ?)`,
		bookID, 2021, 20)
	if err == nil {
		t.Error("se aceptaron dos filas para el mismo libro y año; falta el UNIQUE")
	}
}

func insertAuthor(t *testing.T, db *sql.DB, name string) int64 {
	t.Helper()

	res, err := db.ExecContext(context.Background(),
		`INSERT INTO authors (name) VALUES (?)`, name)
	if err != nil {
		t.Fatalf("insertar autor: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId del autor: %v", err)
	}
	return id
}

func insertBook(t *testing.T, db *sql.DB, authorID int64, name string) int64 {
	t.Helper()

	res, err := db.ExecContext(context.Background(),
		`INSERT INTO books (author_id, name) VALUES (?, ?)`, authorID, name)
	if err != nil {
		t.Fatalf("insertar libro: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId del libro: %v", err)
	}
	return id
}
