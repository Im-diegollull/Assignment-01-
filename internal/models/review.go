package models

import "fmt"

const (
	MinScore = 1
	MaxScore = 5
)

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
