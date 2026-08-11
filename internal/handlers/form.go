package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"bookreviews/internal/models"
)

// form envuelve los valores POST de un request y acumula los errores de parseo.
//
// Los inputs del HTML se llaman igual que el campo del struct de dominio
// (name="NumberOfSales", no name="number_of_sales"): así la clave del input, la
// del mapa de errores y el nombre del campo son la misma, y no hace falta una
// tabla de traducción entre las tres.
type form struct {
	values url.Values
	errs   models.Errors
}

func newForm(r *http.Request) (*form, error) {
	if err := r.ParseForm(); err != nil {
		return nil, fmt.Errorf("handlers: parsear formulario: %w", err)
	}
	return &form{values: r.PostForm, errs: models.Errors{}}, nil
}

// text devuelve el valor sin espacios en los extremos.
func (f *form) text(field string) string {
	return strings.TrimSpace(f.values.Get(field))
}

// integer convierte un campo numérico. Un valor vacío da 0 y deja que la
// validación del dominio decida si eso es aceptable; un valor no numérico sí es
// un error de formulario, porque el dominio no puede distinguirlo de un 0.
func (f *form) integer(field, label string) int {
	raw := f.text(field)
	if raw == "" {
		return 0
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		f.errs[field] = fmt.Sprintf("%s debe ser un número entero.", label)
		return 0
	}
	return value
}

// id convierte la referencia elegida en un <select>.
func (f *form) id(field, label string) int64 {
	raw := f.text(field)
	if raw == "" {
		return 0
	}

	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		f.errs[field] = fmt.Sprintf("%s no es válido.", label)
		return 0
	}
	return value
}

// merge suma los errores de validación del dominio a los de parseo. Los de
// parseo ganan: si el usuario escribió "abc" donde iba un número, ese mensaje
// es más útil que el "no puede ser negativo" que dispararía el 0 resultante.
func (f *form) merge(domainErrors models.Errors) models.Errors {
	for field, message := range domainErrors {
		if _, alreadyFailed := f.errs[field]; !alreadyFailed {
			f.errs[field] = message
		}
	}
	return f.errs
}
