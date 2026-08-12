package models

type Author struct {
	ID              int64
	Name            string
	DateOfBirth     string // ISO 'YYYY-MM-DD', "" si no se informó
	CountryOfOrigin string
	Description     string
}

func (a *Author) Validate() Errors {
	errs := Errors{}
	errs.checkRequired("Name", "El nombre", a.Name, 200)
	errs.checkDate("DateOfBirth", "La fecha de nacimiento", a.DateOfBirth, false)
	errs.checkOptional("CountryOfOrigin", "El país de origen", a.CountryOfOrigin, 100)
	errs.checkOptional("Description", "La descripción", a.Description, 2000)
	return errs
}
