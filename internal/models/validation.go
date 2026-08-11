// Package models contiene los structs de dominio y su validación. No importa
// nada del resto del proyecto: ni SQL, ni HTTP, ni plantillas.
package models

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// DateLayout es el formato en que se guardan las fechas: TEXT ISO-8601.
const DateLayout = "2006-01-02"

// minYear descarta fechas absurdas por tipeo (año 12 en vez de 2012).
const minYear = 1000

// Errors mapea nombre de campo → mensaje. Un mapa vacío significa que el
// registro es válido. Los handlers lo devuelven a la plantilla para re-renderizar
// el formulario con los valores que el usuario ya había escrito.
type Errors map[string]string

// add registra el primer error de un campo; los siguientes se ignoran para no
// mostrar dos mensajes contradictorios sobre lo mismo.
func (e Errors) add(field, message string) {
	if _, exists := e[field]; !exists {
		e[field] = message
	}
}

// checkRequired valida un campo de texto obligatorio.
func (e Errors) checkRequired(field, label, value string, max int) {
	switch {
	case strings.TrimSpace(value) == "":
		e.add(field, fmt.Sprintf("%s es obligatorio.", label))
	case tooLong(value, max):
		e.add(field, fmt.Sprintf("%s no puede superar los %d caracteres.", label, max))
	}
}

// checkOptional valida un campo de texto que puede venir vacío.
func (e Errors) checkOptional(field, label, value string, max int) {
	if tooLong(value, max) {
		e.add(field, fmt.Sprintf("%s no puede superar los %d caracteres.", label, max))
	}
}

// checkDate valida una fecha ISO. Si required es false, el valor vacío pasa.
func (e Errors) checkDate(field, label, value string, required bool) {
	if strings.TrimSpace(value) == "" {
		if required {
			e.add(field, fmt.Sprintf("%s es obligatoria.", label))
		}
		return
	}

	parsed, err := time.Parse(DateLayout, value)
	if err != nil {
		e.add(field, fmt.Sprintf("%s debe tener el formato AAAA-MM-DD.", label))
		return
	}
	e.checkYear(field, label, parsed.Year())
}

// checkYear rechaza años fuera de un rango razonable. Se permite el año próximo
// porque un libro puede tener fecha de publicación futura ya anunciada.
func (e Errors) checkYear(field, label string, year int) {
	max := time.Now().Year() + 1
	if year < minYear || year > max {
		e.add(field, fmt.Sprintf("%s debe estar entre %d y %d.", label, minYear, max))
	}
}

// checkPositiveID exige una referencia elegida en el formulario.
func (e Errors) checkPositiveID(field, label string, id int64) {
	if id <= 0 {
		e.add(field, fmt.Sprintf("Debes seleccionar %s.", label))
	}
}

// checkNotNegative rechaza cantidades negativas.
func (e Errors) checkNotNegative(field, label string, value int) {
	if value < 0 {
		e.add(field, fmt.Sprintf("%s no puede ser negativo.", label))
	}
}

// tooLong cuenta runas y no bytes: "José" son 4 caracteres aunque ocupe 5 bytes.
func tooLong(value string, max int) bool {
	return utf8.RuneCountInString(value) > max
}
