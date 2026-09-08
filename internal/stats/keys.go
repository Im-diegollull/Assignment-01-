package stats

import "fmt"

const (
	KeyAuthorsOverview = "authors:overview"
	KeyTopRated        = "books:top_rated"
	KeyTopSelling      = "books:top_selling"
)

func BookAvgKey(bookID int64) string {
	return fmt.Sprintf("book:avg:%d", bookID)
}
