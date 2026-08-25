package models

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
