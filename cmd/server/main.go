// Command server levanta la aplicación web de reseñas de libros.
package main

import (
	"flag"
	"log/slog"
	"net/http"
	"os"
	"time"

	"bookreviews/internal/handlers"
	"bookreviews/internal/web"
)

func main() {
	addr := flag.String("addr", ":8080", "dirección HTTP de escucha")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if err := run(*addr, logger); err != nil {
		logger.Error("el servidor terminó con error", "error", err)
		os.Exit(1)
	}
}

// run arma el grafo de dependencias y bloquea sirviendo. Está separado de main
// para poder devolver error en vez de llamar a os.Exit desde varios puntos.
func run(addr string, logger *slog.Logger) error {
	handler, err := handlers.New(logger)
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
