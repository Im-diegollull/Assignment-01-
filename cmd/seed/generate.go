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
	minAuthorAge   = 24 // años entre el nacimiento del autor y su primer libro
	maxCalendarEnd = 2026
)

// generator concentra la aleatoriedad en un único *rand.Rand. Todas las
// decisiones se toman en orden y desde esta misma fuente: con la misma semilla,
// el dataset resultante es idéntico bit a bit.
type generator struct {
	rnd *rand.Rand
}

func newGenerator(seed int64) *generator {
	return &generator{rnd: rand.New(rand.NewSource(seed))}
}

// bookPlan lleva el libro junto a los dos rasgos ocultos que gobiernan sus
// reseñas y sus ventas. No se persisten: solo alimentan la generación.
type bookPlan struct {
	book models.Book

	// quality sesga los puntajes de las reseñas (1 a 5). Sin este sesgo por
	// libro, todos los promedios convergerían a 3 y el top 10 de la §5.2 sería
	// un empate masivo decidido por el desempate.
	quality float64

	// popularity es la magnitud de ventas del primer año, en escala
	// logarítmica: la mayoría vende poco y unos pocos venden muchísimo.
	popularity float64
}

// pick elige un elemento al azar de una lista.
func pick[T any](g *generator, options []T) T {
	return options[g.rnd.Intn(len(options))]
}

// intBetween devuelve un entero en [min, max], ambos incluidos.
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

// authorDescription evita construcciones que concuerden en género con el rol:
// "traductora y narradora" y "novelista" conviven en la misma lista, así que la
// plantilla corta la frase antes de cualquier adjetivo.
func (g *generator) authorDescription() string {
	return fmt.Sprintf("%s. Su obra gira en torno a %s. %s",
		capitalize(pick(g, authorRoles)),
		pick(g, authorThemes),
		pick(g, authorTraits))
}

// books reparte los libros entre los autores de forma despareja pero
// garantizando que ninguno quede sin obra: primero se le da un libro a cada
// autor y el resto se sortea con pesos, para que la tabla de la §5.1 tenga
// autores de 1 libro y autores de 15.
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
			// Distribución centrada en 3.4 y recortada: la mayoría de los
			// libros son del montón y unos pocos son muy buenos o muy malos.
			quality: clampFloat(3.4+g.rnd.NormFloat64()*0.9, 1.2, 4.9),
			// Exponencial: pocas superventas, larga cola de libros discretos.
			popularity: g.rnd.ExpFloat64(),
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
	// Varias plantillas empiezan con el personaje, que viene en minúscula
	// ("una traductora"), así que la frase se capitaliza ya armada.
	opening := capitalize(fmt.Sprintf(pick(g, summaryOpenings),
		pick(g, summaryCharacters), pick(g, titlePlaces)))

	return contractPrepositions(strings.Join([]string{
		opening,
		pick(g, summaryMiddles),
		pick(g, summaryClosings),
	}, " "))
}

// contractPrepositions aplica las contracciones obligatorias del español. Los
// lugares del corpus llevan artículo ("el sur", "el kilómetro cero"), así que
// al insertarlos en una plantilla salen frases como "volver a el barrio".
//
// El espacio final de cada patrón es lo que evita tocar "de ella" o "a ellos",
// donde no hay contracción.
func contractPrepositions(text string) string {
	text = strings.ReplaceAll(text, " a el ", " al ")
	return strings.ReplaceAll(text, " de el ", " del ")
}

// publicationYear ubica el libro después de que el autor cumpliera minAuthorAge
// y dentro de la ventana de publicación del dataset.
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

// reviews genera entre 1 y 10 reseñas por libro, con el puntaje sesgado por la
// calidad oculta del libro.
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

// sales genera al menos 5 años consecutivos de ventas por libro, desde su año
// de publicación. Las ventas caen año a año con ruido: el primer año es el
// mejor, como suele pasar con las novedades.
func (g *generator) sales(plans []bookPlan) []models.Sale {
	sales := make([]models.Sale, 0, len(plans)*8)

	for _, plan := range plans {
		startYear := plan.book.PublicationYear()
		span := g.intBetween(minSalesYears, maxSalesYears)
		if end := startYear + span - 1; end > maxCalendarEnd {
			span = maxCalendarEnd - startYear + 1
		}
		if span < minSalesYears {
			span = minSalesYears // nunca por debajo del mínimo del enunciado
		}

		// popularity es exponencial: elevarla convierte esa cola larga en
		// órdenes de magnitud de diferencia entre un libro discreto y un éxito.
		current := 400 + plan.popularity*plan.popularity*9000

		for offset := range span {
			units := int(current * (0.75 + g.rnd.Float64()*0.5))
			if units < 0 {
				units = 0
			}
			sales = append(sales, models.Sale{
				BookID: plan.book.ID,
				Year:   startYear + offset,
				Sales:  units,
			})
			current *= 0.45 + g.rnd.Float64()*0.3 // decae entre 45% y 75% por año
		}
	}
	return sales
}

// dateIn arma una fecha ISO dentro del año dado. El día se limita a 28 para no
// tener que mirar el mes ni los años bisiestos.
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

// capitalize pone en mayúscula la primera letra. Convierte a []rune en vez de
// cortar con s[:1] porque las palabras del corpus llevan acentos: en UTF-8 un
// carácter acentuado ocupa dos bytes y el corte por byte lo partiría al medio.
func capitalize(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}
