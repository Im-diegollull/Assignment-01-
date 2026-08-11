// Package database abre la conexión a SQLite y aplica el esquema. Es la capa
// más baja: no conoce ni el dominio ni HTTP.
package database

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

// No borrar la línea de abajo: //go:embed es una directiva del compilador, no un
// comentario. Sin ella schema queda vacío y Migrate no crea ninguna tabla.
//
//go:embed schema.sql
var schema string

// DefaultPath es la ubicación del archivo de base de datos por omisión.
const DefaultPath = "data/app.db"

func Open(ctx context.Context, path string) (*sql.DB, error) {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("database: crear directorio %s: %w", dir, err)
		}
	}

	db, err := sql.Open("sqlite", dsn(path))
	if err != nil {
		return nil, fmt.Errorf("database: abrir %s: %w", path, err)
	}

	db.SetMaxOpenConns(1)

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("database: conectar a %s: %w", path, err)
	}

	if err := verifyPragmas(ctx, db); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

func dsn(path string) string {
	return "file:" + path +
		"?_pragma=journal_mode(WAL)" +
		"&_pragma=foreign_keys(1)" +
		"&_pragma=busy_timeout(5000)"
}

func verifyPragmas(ctx context.Context, db *sql.DB) error {
	var foreignKeys int
	if err := db.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		return fmt.Errorf("database: leer pragma foreign_keys: %w", err)
	}
	if foreignKeys != 1 {
		return fmt.Errorf("database: el pragma foreign_keys quedó en %d; las FK no se estarían aplicando", foreignKeys)
	}
	return nil
}

func Migrate(ctx context.Context, db *sql.DB) error {
	// Un schema vacío significa que se perdió la directiva //go:embed. Sin esta
	// guarda, Migrate "tendría éxito" sin crear ninguna tabla.
	if strings.TrimSpace(schema) == "" {
		return fmt.Errorf("database: el esquema embebido está vacío; falta la directiva //go:embed schema.sql")
	}
	if _, err := db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("database: aplicar esquema: %w", err)
	}
	return nil
}
