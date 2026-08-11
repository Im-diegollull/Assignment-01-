package models

import (
	"strconv"
	"time"
)

// Book es un libro, siempre de un único autor (ver la decisión 1-N en el DEVLOG).
type Book struct {
	ID              int64
	AuthorID        int64
	Name            string
	Summary         string
	PublicationDate string // ISO 'YYYY-MM-DD'
	// NumberOfSales duplica SUM(sales_by_year.sales). El store lo recalcula
	// cada vez que cambian las ventas por año de este libro.
	NumberOfSales int
}

// La fecha de publicación es obligatoria: la tabla del top 50 (§5.3) necesita el
// año de publicación para decidir si el libro entró al top 5 de ese año.
func (b *Book) Validate() Errors {
	errs := Errors{}
	errs.checkPositiveID("AuthorID", "un autor", b.AuthorID)
	errs.checkRequired("Name", "El nombre", b.Name, 300)
	errs.checkOptional("Summary", "El resumen", b.Summary, 4000)
	errs.checkDate("PublicationDate", "La fecha de publicación", b.PublicationDate, true)
	errs.checkNotNegative("NumberOfSales", "El número de ventas", b.NumberOfSales)
	return errs
}

// PublicationYear extrae el año de la fecha de publicación. Devuelve 0 si la
// fecha está vacía o mal formada.
func (b *Book) PublicationYear() int {
	parsed, err := time.Parse(DateLayout, b.PublicationDate)
	if err != nil {
		return 0
	}
	return parsed.Year()
}

// String identifica el libro en los desplegables de reseñas y ventas.
func (b *Book) String() string {
	if year := b.PublicationYear(); year > 0 {
		return b.Name + " (" + strconv.Itoa(year) + ")"
	}
	return b.Name
}
