package main

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
	"unicode"

	"bookreviews/internal/models"
)

// Parámetros del dataset pedido por el enunciado.
const (
	numAuthors     = 50
	numBooks       = 300
	minReviews     = 1
	maxReviews     = 10
	minSalesYears  = 5
	maxSalesYears  = 12
	firstPubYear   = 1962
	lastPubYear    = 2020
	minBirthYear   = 1900
	lastBirthYear  = 1985
	minAuthorAge   = 24
	maxCalendarEnd = 2026

	slowBurnerShare = 0.18
)

type generator struct {
	rnd *rand.Rand
}

func newGenerator(seed int64) *generator {
	return &generator{rnd: rand.New(rand.NewSource(seed))}
}

type bookPlan struct {
	book models.Book

	quality float64

	popularity float64

	slowBurner bool
}

func pick[T any](g *generator, options []T) T {
	return options[g.rnd.Intn(len(options))]
}

func (g *generator) intBetween(min, max int) int {
	if max <= min {
		return min
	}
	return min + g.rnd.Intn(max-min+1)
}

func (g *generator) authors() []models.Author {
	authors := make([]models.Author, 0, numAuthors)
	used := make(map[string]bool, numAuthors)

	for len(authors) < numAuthors {
		name := pick(g, firstNames) + " " + pick(g, lastNames)
		if used[name] {
			continue // dos autores con el mismo nombre confundirían las tablas
		}
		used[name] = true

		birthYear := g.intBetween(minBirthYear, lastBirthYear)
		authors = append(authors, models.Author{
			Name:            name,
			DateOfBirth:     g.dateIn(birthYear),
			CountryOfOrigin: pick(g, countries),
			Description:     g.authorDescription(),
		})
	}
	return authors
}

func (g *generator) authorDescription() string {
	return fmt.Sprintf("%s. Su obra gira en torno a %s. %s",
		capitalize(pick(g, authorRoles)),
		pick(g, authorThemes),
		pick(g, authorTraits))
}

func (g *generator) books(authors []models.Author) []bookPlan {
	weighted := make([]int, 0, numBooks)
	for i := range authors {
		weight := g.intBetween(1, 12)
		for range weight {
			weighted = append(weighted, i)
		}
	}

	plans := make([]bookPlan, 0, numBooks)
	usedTitles := make(map[string]bool, numBooks)

	for i := range numBooks {
		authorIndex := i
		if i >= len(authors) {
			authorIndex = weighted[g.rnd.Intn(len(weighted))]
		}
		author := authors[authorIndex]

		title := g.title()
		for usedTitles[title] {
			title = g.title()
		}
		usedTitles[title] = true

		plans = append(plans, bookPlan{
			book: models.Book{
				AuthorID:        author.ID,
				Name:            title,
				Summary:         g.summary(),
				PublicationDate: g.dateIn(g.publicationYear(author)),
			},

			quality: clampFloat(3.4+g.rnd.NormFloat64()*0.9, 1.2, 4.9),

			popularity: g.rnd.ExpFloat64(),
			slowBurner: g.rnd.Float64() < slowBurnerShare,
		})
	}
	return plans
}

func (g *generator) title() string {
	switch g.rnd.Intn(4) {
	case 0:
		return fmt.Sprintf("El %s de %s", pick(g, titleNouns), pick(g, titleModifiers))
	case 1:
		return fmt.Sprintf("%s %s", pick(g, titleAdjectives), pick(g, titleNouns))
	case 2:
		return fmt.Sprintf("Cartas desde %s", pick(g, titlePlaces))
	default:
		return fmt.Sprintf("%s en %s", capitalize(pick(g, titleNouns)), pick(g, titlePlaces))
	}
}

func (g *generator) summary() string {

	opening := capitalize(fmt.Sprintf(pick(g, summaryOpenings),
		pick(g, summaryCharacters), pick(g, titlePlaces)))

	return contractPrepositions(strings.Join([]string{
		opening,
		pick(g, summaryMiddles),
		pick(g, summaryClosings),
	}, " "))
}

func contractPrepositions(text string) string {
	text = strings.ReplaceAll(text, " a el ", " al ")
	return strings.ReplaceAll(text, " de el ", " del ")
}

func (g *generator) publicationYear(author models.Author) int {
	earliest := firstPubYear
	if birth := yearOf(author.DateOfBirth); birth > 0 && birth+minAuthorAge > earliest {
		earliest = birth + minAuthorAge
	}
	if earliest > lastPubYear {
		earliest = lastPubYear
	}
	return g.intBetween(earliest, lastPubYear)
}

func (g *generator) reviews(plans []bookPlan) []models.Review {
	reviews := make([]models.Review, 0, len(plans)*5)

	for _, plan := range plans {
		count := g.intBetween(minReviews, maxReviews)
		for range count {
			score := int(math.Round(clampFloat(plan.quality+g.rnd.NormFloat64()*0.8,
				float64(models.MinScore), float64(models.MaxScore))))

			reviews = append(reviews, models.Review{
				BookID: plan.book.ID,
				Text:   pick(g, reviewsByScore[score]),
				Score:  score,
				// Las reseñas positivas tienden a juntar más votos.
				Upvotes: g.rnd.Intn(40 + score*score*18),
			})
		}
	}
	return reviews
}

func (g *generator) sales(plans []bookPlan) []models.Sale {
	sales := make([]models.Sale, 0, len(plans)*8)

	for _, plan := range plans {
		startYear := plan.book.PublicationYear()
		span := g.intBetween(minSalesYears, maxSalesYears)
		if end := startYear + span - 1; end > maxCalendarEnd {
			span = maxCalendarEnd - startYear + 1
		}
		if span < minSalesYears {
			span = minSalesYears
		}

		magnitude := 400 + plan.popularity*plan.popularity*9000

		peak := 0
		if plan.slowBurner {
			peak = min(g.intBetween(2, 5), span-2)
		}

		for offset := range span {
			units := int(magnitude * salesCurve(offset, peak) * (0.8 + g.rnd.Float64()*0.4))
			if units < 0 {
				units = 0
			}
			sales = append(sales, models.Sale{
				BookID: plan.book.ID,
				Year:   startYear + offset,
				Sales:  units,
			})
		}
	}
	return sales
}

func salesCurve(offset, peak int) float64 {
	if offset < peak {
		return 0.12 + 0.88*float64(offset)/float64(peak)
	}
	return math.Pow(0.6, float64(offset-peak))
}

func (g *generator) dateIn(year int) string {
	return fmt.Sprintf("%04d-%02d-%02d", year, g.intBetween(1, 12), g.intBetween(1, 28))
}

func yearOf(isoDate string) int {
	var year int
	if _, err := fmt.Sscanf(isoDate, "%4d", &year); err != nil {
		return 0
	}
	return year
}

func clampFloat(value, min, max float64) float64 {
	return math.Min(math.Max(value, min), max)
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}
