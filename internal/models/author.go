package models

// Author es un autor de libros.
//
// Los campos opcionales son string y no *string ni sql.NullString: el store
// lee las columnas con COALESCE y convierte los NULL en cadena vacía, así que
// una cadena vacía representa "sin dato" en todo el proyecto. Evita punteros en
// las plantillas y mantiene models sin importar database/sql.
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
