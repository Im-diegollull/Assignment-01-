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

	for i := range plans {
		plans[i].book.ID = books[i].ID
	}
	logger.Info("libros insertados", "cantidad", len(books))

	reviews := gen.reviews(plans)
	if err := st.Reviews.CreateMany(ctx, reviews); err != nil {
		return err
	}
	logger.Info("reseñas insertadas", "cantidad", len(reviews))

	sales := gen.sales(plans)
	if err := st.Sales.CreateMany(ctx, sales); err != nil {
		return err
	}
	logger.Info("ventas insertadas", "cantidad", len(sales))

	logger.Info("seed completo", "semilla", randSeed)
	return nil
}
