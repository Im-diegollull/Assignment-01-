package models

// Sale son las ventas de un libro en un año calendario. La base impone
// UNIQUE(book_id, year): un libro tiene a lo sumo una fila por año.
type Sale struct {
	ID     int64
	BookID int64
	Year   int
	Sales  int
}

func (s *Sale) Validate() Errors {
	errs := Errors{}
	errs.checkPositiveID("BookID", "un libro", s.BookID)
	errs.checkYear("Year", "El año", s.Year)
	errs.checkNotNegative("Sales", "Las ventas", s.Sales)
	return errs
}
