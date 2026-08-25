// Package models contiene los structs de dominio y su validación. No importa
// nada del resto del proyecto: ni SQL, ni HTTP, ni plantillas.
package models

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

const DateLayout = "2006-01-02"

const minYear = 1000

type Errors map[string]string

func (e Errors) add(field, message string) {
	if _, exists := e[field]; !exists {
		e[field] = message
	}
}

func (e Errors) checkRequired(field, label, value string, max int) {
	switch {
	case strings.TrimSpace(value) == "":
		e.add(field, fmt.Sprintf("%s es obligatorio.", label))
	case tooLong(value, max):
		e.add(field, fmt.Sprintf("%s no puede superar los %d caracteres.", label, max))
	}
}

func (e Errors) checkOptional(field, label, value string, max int) {
	if tooLong(value, max) {
		e.add(field, fmt.Sprintf("%s no puede superar los %d caracteres.", label, max))
	}
}

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

func (e Errors) checkYear(field, label string, year int) {
	max := time.Now().Year() + 1
	if year < minYear || year > max {
		e.add(field, fmt.Sprintf("%s debe estar entre %d y %d.", label, minYear, max))
	}
}

func (e Errors) checkPositiveID(field, label string, id int64) {
	if id <= 0 {
		e.add(field, fmt.Sprintf("Debes seleccionar %s.", label))
	}
}

func (e Errors) checkNotNegative(field, label string, value int) {
	if value < 0 {
		e.add(field, fmt.Sprintf("%s no puede ser negativo.", label))
	}
}

func tooLong(value string, max int) bool {
	return utf8.RuneCountInString(value) > max
}
