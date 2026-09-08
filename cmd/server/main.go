// Command server levanta la aplicación web de reseñas de libros.
package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"time"

	"bookreviews/internal/cache"
	"bookreviews/internal/database"
	"bookreviews/internal/handlers"
	"bookreviews/internal/search"
	"bookreviews/internal/stats"
	"bookreviews/internal/store"
	"bookreviews/internal/web"
)

func main() {
	var (
		addr        = flag.String("addr", ":"+envOrDefault("PORT", "8080"), "dirección HTTP de escucha")
		dbPath      = flag.String("db", envOrDefault("DB_PATH", database.DefaultPath), "ruta del archivo SQLite")
		migrateOnly = flag.Bool("migrate", false, "aplicar el esquema y salir, sin levantar el servidor")
	)
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if err := run(*addr, *dbPath, *migrateOnly, logger); err != nil {
		logger.Error("el servidor terminó con error", "error", err)
		os.Exit(1)
	}
}

// run arma el grafo de dependencias y bloquea sirviendo. Está separado de main
// para poder devolver error en vez de llamar a os.Exit desde varios puntos.
func run(addr, dbPath string, migrateOnly bool, logger *slog.Logger) error {
	ctx := context.Background()

	db, err := database.Open(ctx, dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	logger.Info("base de datos abierta", "path", dbPath)

	// El DDL es idempotente, así que se aplica siempre: una copia recién
	// clonada del repo arranca sin pasos previos.
	if err := database.Migrate(ctx, db); err != nil {
		return err
	}
	if migrateOnly {
		logger.Info("esquema aplicado, saliendo por -migrate")
		return nil
	}

	st := store.New(db)

	cacheClient := cache.Noop()
	if redisAddr := envOrDefault("REDIS_ADDR", ""); redisAddr != "" {
		redisCache, err := cache.OpenRedis(ctx, redisAddr)
		if err != nil {
			return err
		}
		defer redisCache.Close()
		cacheClient = redisCache
		logger.Info("cache Redis conectado", "addr", redisAddr)
	} else {
		logger.Info("cache desactivado: REDIS_ADDR vacío")
	}

	var engine search.Engine = search.SQL{Store: st}
	if url := envOrDefault("OPENSEARCH_URL", ""); url != "" {
		osEngine, err := search.Open(ctx, url, st)
		if err != nil {
			return err
		}
		if err := osEngine.ReindexAll(ctx); err != nil {
			return err
		}
		engine = osEngine
		logger.Info("OpenSearch conectado e índice sincronizado", "url", url)
	} else {
		logger.Info("búsqueda SQL: OPENSEARCH_URL vacío")
	}

	handler, err := handlers.New(logger, st, stats.New(st, cacheClient, logger), engine)
	if err != nil {
		return err
	}

	srv := &http.Server{
		Addr: addr,
		Handler: web.Chain(handler.Routes(),
			web.RecoverPanic(logger),
			web.LogRequests(logger),
		),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  time.Minute,
	}

	logger.Info("servidor iniciado", "addr", addr)
	return srv.ListenAndServe()
}

// envOrDefault lee una variable de entorno y cae al default si no está seteada
// o está vacía. Los flags (-addr, -db) siguen pudiendo sobreescribirla: el
// entorno solo cambia el valor por defecto del flag.
func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
