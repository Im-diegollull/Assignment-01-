package models

import "fmt"

// Límites del puntaje. La base repite la regla en un CHECK como red de
// seguridad, pero la validación de negocio vive acá y es la que produce el
// mensaje que ve el usuario.
const (
	MinScore = 1
	MaxScore = 5
)

// Review es una reseña de un libro.
//
// El texto se llama Text y no Review, aunque la columna sea "review": un campo
// homónimo del struct que lo contiene queda tapado al embeber el struct en un
// view model (r.Review resolvería al struct embebido y no al texto).
type Review struct {
	ID      int64
	BookID  int64
	Text    string
	Score   int
	Upvotes int
}

func (r *Review) Validate() Errors {
	errs := Errors{}
	errs.checkPositiveID("BookID", "un libro", r.BookID)
	errs.checkRequired("Text", "El texto de la reseña", r.Text, 4000)
	if r.Score < MinScore || r.Score > MaxScore {
		errs.add("Score", fmt.Sprintf("El puntaje debe estar entre %d y %d.", MinScore, MaxScore))
	}
	errs.checkNotNegative("Upvotes", "Los up-votes", r.Upvotes)
	return errs
}
