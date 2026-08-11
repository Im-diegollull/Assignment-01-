// Command seed llena la base con datos de prueba generados proceduralmente.
//
// Los datos son inventados y se arman combinando listas de vocabulario: no se
// consulta ninguna API externa. Con la misma semilla, el dataset es idéntico
// entre corridas, así que dos personas del grupo ven exactamente los mismos
// rankings en las tablas de la §5.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"bookreviews/internal/database"
	"bookreviews/internal/models"
	"bookreviews/internal/store"
)

func main() {
	var (
		dbPath   = flag.String("db", database.DefaultPath, "ruta del archivo SQLite")
		reset    = flag.Bool("reset", false, "vaciar las tablas antes de sembrar")
		randSeed = flag.Int64("seed", 42, "semilla del generador; la misma semilla produce los mismos datos")
	)
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if err := run(*dbPath, *reset, *randSeed, logger); err != nil {
		logger.Error("el seed falló", "error", err)
		os.Exit(1)
	}
}

func run(dbPath string, reset bool, randSeed int64, logger *slog.Logger) error {
	ctx := context.Background()

	db, err := database.Open(ctx, dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := database.Migrate(ctx, db); err != nil {
		return err
	}

	st := store.New(db)
	if err := prepare(ctx, st, reset, logger); err != nil {
		return err
	}

	return populate(ctx, st, randSeed, logger)
}

// prepare deja la base vacía. Sin --reset se niega a sembrar sobre datos
// existentes: correr el seed dos veces sin querer duplicaría los 300 libros.
func prepare(ctx context.Context, st *store.Store, reset bool, logger *slog.Logger) error {
	counts, err := st.Count(ctx)
	if err != nil {
		return err
	}
	if counts.Empty() {
		return nil
	}

	if !reset {
		return fmt.Errorf("la base ya tiene datos (%d autores, %d libros, %d reseñas, %d ventas); "+
			"usa --reset para vaciarla antes de sembrar",
			counts.Authors, counts.Books, counts.Reviews, counts.Sales)
	}

	logger.Info("vaciando las tablas", "autores", counts.Authors, "libros", counts.Books,
		"reseñas", counts.Reviews, "ventas", counts.Sales)
	return st.Reset(ctx)
}

// populate genera e inserta el dataset. El orden respeta las FK: los libros
// necesitan el id de su autor, y las reseñas y ventas el id de su libro, así
// que cada lote se inserta antes de generar el siguiente.
func populate(ctx context.Context, st *store.Store, randSeed int64, logger *slog.Logger) error {
	gen := newGenerator(randSeed)

	authors := gen.authors()
	if err := st.Authors.CreateMany(ctx, authors); err != nil {
		return err
	}
	logger.Info("autores insertados", "cantidad", len(authors))

	plans := gen.books(authors)
	books := make([]models.Book, len(plans))
	for i := range plans {
		books[i] = plans[i].book
	}
	if err := st.Books.CreateMany(ctx, books); err != nil {
		return err
	}
	// CreateMany completó los ids; los planes tienen que verlos para poder
	// referenciarlos desde las reseñas y las ventas.
	for i := range plans {
		plans[i].book.ID = books[i].ID
	}
	logger.Info("libros insertados", "cantidad", len(books))

	reviews := gen.reviews(plans)
	if err := st.Reviews.CreateMany(ctx, reviews); err != nil {
		return err
	}
	logger.Info("reseñas insertadas", "cantidad", len(reviews))

	// CreateMany de ventas recalcula books.number_of_sales en la misma
	// transacción, así el campo denormalizado queda consistente con la suma.
	sales := gen.sales(plans)
	if err := st.Sales.CreateMany(ctx, sales); err != nil {
		return err
	}
	logger.Info("ventas insertadas", "cantidad", len(sales))

	logger.Info("seed completo", "semilla", randSeed)
	return nil
}
