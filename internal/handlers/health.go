package handlers

import "net/http"

// healthz es el endpoint de salud para las probes de Kubernetes (y para
// docker compose/local). Hace un ping real a la base: esta app no tiene
// ninguna lógica que tenga sentido sin ella, así que "viva pero sin poder
// llegar a SQLite" no es un estado en el que valga la pena seguir
// recibiendo tráfico.
func (h *Handler) healthz(w http.ResponseWriter, r *http.Request) {
	if err := h.store.Ping(r.Context()); err != nil {
		h.logger.Error("healthz: base de datos no responde", "error", err)
		http.Error(w, "no disponible", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
}
