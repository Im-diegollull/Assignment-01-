package handlers

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"bookreviews/internal/store"
)

const maxImageBytes = 5 << 20

func (h *Handler) uploadBookImage(w http.ResponseWriter, r *http.Request) {
	h.uploadImage(w, r, true)
}

func (h *Handler) uploadAuthorImage(w http.ResponseWriter, r *http.Request) {
	h.uploadImage(w, r, false)
}

func (h *Handler) uploadImage(w http.ResponseWriter, r *http.Request, book bool) {
	id, ok := pathID(r)
	if !ok {
		h.notFound(w)
		return
	}
	var err error
	if book {
		_, err = h.store.Books.Get(r.Context(), id)
	} else {
		_, err = h.store.Authors.Get(r.Context(), id)
	}
	if errors.Is(err, store.ErrNotFound) {
		h.notFound(w)
		return
	}
	if err != nil {
		h.serverError(w, err)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxImageBytes+(64<<10))
	if err := r.ParseMultipartForm(maxImageBytes + (64 << 10)); err != nil {
		http.Error(w, "Imagen o formulario demasiado grande", http.StatusBadRequest)
		return
	}
	defer r.MultipartForm.RemoveAll()
	file, _, err := r.FormFile("Image")
	if err != nil {
		http.Error(w, "Selecciona una imagen", http.StatusBadRequest)
		return
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, maxImageBytes+1))
	if err != nil || len(raw) > maxImageBytes {
		http.Error(w, "La imagen no puede superar 5 MiB", http.StatusRequestEntityTooLarge)
		return
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil || config.Width < 1 || config.Height < 1 || int64(config.Width)*int64(config.Height) > 20_000_000 {
		http.Error(w, "Imagen PNG, JPEG o GIF no valida", http.StatusUnprocessableEntity)
		return
	}
	if _, _, err := image.Decode(bytes.NewReader(raw)); err != nil {
		http.Error(w, "Imagen incompleta", http.StatusUnprocessableEntity)
		return
	}
	name := fmt.Sprintf("%x.%s", sha256.Sum256(raw), format)
	if err := writeImage(h.mediaRoot, name, raw); err != nil {
		h.serverError(w, err)
		return
	}
	url := authorURL(id)
	if book {
		err = h.store.Books.SetImage(r.Context(), id, name)
		url = bookURL(id)
	} else {
		err = h.store.Authors.SetImage(r.Context(), id, name)
	}
	if err != nil {
		h.serverError(w, err)
		return
	}
	h.redirect(w, r, url)
}

func writeImage(root, name string, raw []byte) error {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	file, err := os.CreateTemp(root, ".upload-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(raw); err != nil {
		file.Close()
		return err
	}
	if err := file.Chmod(0o644); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), filepath.Join(root, name))
}
