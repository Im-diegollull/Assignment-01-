package handlers

import "net/http"

// home muestra el índice con los accesos a los CRUD y a las tablas.
func (h *Handler) home(w http.ResponseWriter, r *http.Request) {
	h.render(w, http.StatusOK, "home.html", nil)
}
