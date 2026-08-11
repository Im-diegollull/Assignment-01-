// Package web contiene middleware HTTP genérico, independiente de los
// recursos concretos de la aplicación.
package web

import (
	"log/slog"
	"net/http"
	"time"
)

// Middleware es la forma que ya usa la stdlib para decorar handlers.
type Middleware func(http.Handler) http.Handler

// Chain aplica los middleware en el orden en que se pasan: el primero de la
// lista es el más externo, es decir el primero en ver el request.
func Chain(h http.Handler, middleware ...Middleware) http.Handler {
	for i := len(middleware) - 1; i >= 0; i-- {
		h = middleware[i](h)
	}
	return h
}

// LogRequests registra método, ruta, status y duración de cada request.
func LogRequests(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)
			logger.Info("request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rec.status,
				"duration", time.Since(start).Round(time.Microsecond),
			)
		})
	}
}

// RecoverPanic evita que un panic en un handler mate al proceso completo y lo
// convierte en un 500.
func RecoverPanic(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					logger.Error("panic recuperado", "path", r.URL.Path, "panic", rec)
					w.Header().Set("Connection", "close")
					http.Error(w, "Error interno del servidor", http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// statusRecorder recuerda el status escrito para poder registrarlo.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(status int) {
	s.status = status
	s.ResponseWriter.WriteHeader(status)
}
