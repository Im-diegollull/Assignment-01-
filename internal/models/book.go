package models

import (
	"strconv"
	"time"
)

type Book struct {
	ID              int64
	AuthorID        int64
	Name            string
	Summary         string
	PublicationDate string // ISO 'YYYY-MM-DD'

	NumberOfSales int
	ImagePath     string
}

func (b *Book) Validate() Errors {
	errs := Errors{}
	errs.checkPositiveID("AuthorID", "un autor", b.AuthorID)
	errs.checkRequired("Name", "El nombre", b.Name, 300)
	errs.checkOptional("Summary", "El resumen", b.Summary, 4000)
	errs.checkDate("PublicationDate", "La fecha de publicación", b.PublicationDate, true)
	errs.checkNotNegative("NumberOfSales", "El número de ventas", b.NumberOfSales)
	return errs
}

func (b *Book) PublicationYear() int {
	parsed, err := time.Parse(DateLayout, b.PublicationDate)
	if err != nil {
		return 0
	}
	return parsed.Year()
}

func (b *Book) String() string {
	if year := b.PublicationYear(); year > 0 {
		return b.Name + " (" + strconv.Itoa(year) + ")"
	}
	return b.Name
}
