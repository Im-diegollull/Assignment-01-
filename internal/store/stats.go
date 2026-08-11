package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// Este archivo tiene las tres consultas de reporte del enunciado. Las filas que
// devuelven no son entidades de dominio sino view models: mezclan datos de
// varias tablas con valores calculados, y no tendría sentido cargar
// models.Author o models.Book con ellos.

// ---------------------------------------------------------------------------
// 1. Tabla de autores: libros publicados, score promedio y ventas totales,
//    con orden y filtro por cada columna.
// ---------------------------------------------------------------------------

type AuthorStatsRow struct {
	ID         int64
	Name       string
	BooksCount int
	// AvgScore es NULL para un autor sin reseñas en ninguno de sus libros; no
	// es lo mismo que un promedio de 0, así que la vista lo muestra como "—".
	AvgScore   sql.NullFloat64
	TotalSales int
}

// Expresiones de las columnas calculadas. Se nombran una sola vez porque las
// usan tanto el WHERE como el ORDER BY, y en SQL estándar el WHERE no puede
// referirse a los alias del SELECT.
const (
	exprBooksCount = "COALESCE(bs.books_count, 0)"
	exprAvgScore   = "rs.avg_score"
	exprTotalSales = "COALESCE(bs.total_sales, 0)"
)

// authorSortColumns es la whitelist de ordenamiento. ORDER BY no admite
// placeholders, así que la única forma segura de ordenar por una columna que
// elige el usuario es traducir su nombre con un mapa: lo que no está en el
// mapa no llega al SQL.
var authorSortColumns = map[string]string{
	"name":        "a.name COLLATE NOCASE",
	"books":       exprBooksCount,
	"avg_score":   exprAvgScore,
	"total_sales": exprTotalSales,
}

// AuthorFilter son los filtros y el orden de la tabla de autores. Los punteros
// distinguen "sin filtro" de "filtrar por 0".
type AuthorFilter struct {
	NameLike string
	MinBooks *int
	MaxBooks *int
	MinScore *float64
	MaxScore *float64
	MinSales *int
	MaxSales *int

	SortBy string // clave de authorSortColumns
	Dir    string // "asc" o "desc"
}

// conditions arma los fragmentos del WHERE con sus argumentos. Es una función
// pura: no toca la base y se puede testear sola.
//
// Los valores del usuario nunca se concatenan al SQL; van siempre como
// argumentos de un placeholder ?.
func (f AuthorFilter) conditions() ([]string, []any) {
	var fragments []string
	var args []any

	if name := strings.TrimSpace(f.NameLike); name != "" {
		fragments = append(fragments, "a.name LIKE ?")
		args = append(args, "%"+name+"%")
	}
	if f.MinBooks != nil {
		fragments = append(fragments, exprBooksCount+" >= ?")
		args = append(args, *f.MinBooks)
	}
	if f.MaxBooks != nil {
		fragments = append(fragments, exprBooksCount+" <= ?")
		args = append(args, *f.MaxBooks)
	}
	// Un filtro por score deja fuera a los autores sin reseñas: su promedio es
	// NULL y NULL >= 3 no es verdadero. Es el comportamiento correcto — no se
	// puede afirmar que cumplan el filtro — pero conviene tenerlo presente.
	if f.MinScore != nil {
		fragments = append(fragments, exprAvgScore+" >= ?")
		args = append(args, *f.MinScore)
	}
	if f.MaxScore != nil {
		fragments = append(fragments, exprAvgScore+" <= ?")
		args = append(args, *f.MaxScore)
	}
	if f.MinSales != nil {
		fragments = append(fragments, exprTotalSales+" >= ?")
		args = append(args, *f.MinSales)
	}
	if f.MaxSales != nil {
		fragments = append(fragments, exprTotalSales+" <= ?")
		args = append(args, *f.MaxSales)
	}
	return fragments, args
}

// orderBy traduce la columna pedida usando la whitelist y valida la dirección.
// El id final desempata para que el orden sea estable entre corridas.
func (f AuthorFilter) orderBy() string {
	column, known := authorSortColumns[f.SortBy]
	if !known {
		column = authorSortColumns["name"]
	}

	direction := "ASC"
	if strings.EqualFold(f.Dir, "desc") {
		direction = "DESC"
	}
	return column + " " + direction + ", a.id"
}

// authorStatsSQL agrega cada métrica por separado y recién después las une.
//
// Hacerlo en un solo JOIN de authors, books y reviews estaría mal: una fila por
// cada combinación libro-reseña multiplicaría SUM(number_of_sales) por la
// cantidad de reseñas de cada libro. Los dos CTE colapsan cada lado a una fila
// por autor antes del LEFT JOIN, y así no hay fan-out.
//
// Los LEFT JOIN son los que dejan aparecer a los autores sin libros y a los que
// tienen libros pero ninguna reseña.
const authorStatsSQL = `
WITH book_stats AS (
  SELECT author_id,
         COUNT(*) AS books_count,
         COALESCE(SUM(number_of_sales), 0) AS total_sales
  FROM books
  GROUP BY author_id
),
review_stats AS (
  SELECT b.author_id, AVG(r.score) AS avg_score
  FROM reviews r
  JOIN books b ON b.id = r.book_id
  GROUP BY b.author_id
)
SELECT a.id,
       a.name,
       ` + exprBooksCount + `,
       ` + exprAvgScore + `,
       ` + exprTotalSales + `
FROM authors a
LEFT JOIN book_stats   bs ON bs.author_id = a.id
LEFT JOIN review_stats rs ON rs.author_id = a.id`

// AuthorStats devuelve la tabla de autores con sus métricas, filtrada y
// ordenada. No pagina: son 50 autores.
func (s *AuthorStore) AuthorStats(ctx context.Context, filter AuthorFilter) ([]AuthorStatsRow, error) {
	query := authorStatsSQL
	fragments, args := filter.conditions()
	if len(fragments) > 0 {
		query += "\nWHERE " + strings.Join(fragments, "\n  AND ")
	}
	query += "\nORDER BY " + filter.orderBy()

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("store: tabla de autores: %w", err)
	}
	defer rows.Close()

	var stats []AuthorStatsRow
	for rows.Next() {
		var row AuthorStatsRow
		if err := rows.Scan(&row.ID, &row.Name, &row.BooksCount,
			&row.AvgScore, &row.TotalSales); err != nil {
			return nil, fmt.Errorf("store: tabla de autores: %w", err)
		}
		stats = append(stats, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: tabla de autores: %w", err)
	}
	return stats, nil
}

// ---------------------------------------------------------------------------
// 2. Top 10 libros mejor evaluados, con su mejor y su peor reseña.
// ---------------------------------------------------------------------------

type TopRatedBookRow struct {
	BookID       int64
	BookName     string
	AuthorID     int64
	AuthorName   string
	AvgScore     float64
	ReviewsCount int

	BestScore   int
	BestReview  string
	WorstScore  int
	WorstReview string
}

// topRatedBooksSQL usa funciones de ventana (SQLite ≥ 3.25) para numerar las
// reseñas de cada libro por puntaje, de mayor a menor y de menor a mayor, y se
// queda con la primera de cada orden.
//
// Si un libro tiene una sola reseña, su mejor y su peor reseña son la misma
// fila. Es correcto y no hay que "arreglarlo".
//
// El id del libro desempata en los tres ORDER BY para que el ranking sea
// estable: sin él, dos libros con el mismo promedio y la misma cantidad de
// reseñas podrían intercambiarse entre corridas.
const topRatedBooksSQL = `
WITH scored AS (
  SELECT book_id,
         AVG(score) AS avg_score,
         COUNT(*)   AS reviews_count
  FROM reviews
  GROUP BY book_id
),
top AS (
  SELECT * FROM scored
  ORDER BY avg_score DESC, reviews_count DESC, book_id
  LIMIT ?
),
ranked AS (
  SELECT r.book_id, r.review, r.score,
         ROW_NUMBER() OVER (
           PARTITION BY r.book_id ORDER BY r.score DESC, r.upvotes DESC, r.id
         ) AS rn_best,
         ROW_NUMBER() OVER (
           PARTITION BY r.book_id ORDER BY r.score ASC, r.upvotes DESC, r.id
         ) AS rn_worst
  FROM reviews r
  WHERE r.book_id IN (SELECT book_id FROM top)
)
SELECT b.id, b.name, a.id, a.name,
       t.avg_score, t.reviews_count,
       COALESCE(best.score, 0),  COALESCE(best.review, ''),
       COALESCE(worst.score, 0), COALESCE(worst.review, '')
FROM top t
JOIN books   b ON b.id = t.book_id
JOIN authors a ON a.id = b.author_id
LEFT JOIN ranked best  ON best.book_id  = t.book_id AND best.rn_best   = 1
LEFT JOIN ranked worst ON worst.book_id = t.book_id AND worst.rn_worst = 1
ORDER BY t.avg_score DESC, t.reviews_count DESC, b.id`

// TopRated devuelve los limit libros mejor evaluados de todos los tiempos.
func (s *BookStore) TopRated(ctx context.Context, limit int) ([]TopRatedBookRow, error) {
	rows, err := s.db.QueryContext(ctx, topRatedBooksSQL, limit)
	if err != nil {
		return nil, fmt.Errorf("store: top de libros mejor evaluados: %w", err)
	}
	defer rows.Close()

	var books []TopRatedBookRow
	for rows.Next() {
		var row TopRatedBookRow
		if err := rows.Scan(&row.BookID, &row.BookName, &row.AuthorID, &row.AuthorName,
			&row.AvgScore, &row.ReviewsCount,
			&row.BestScore, &row.BestReview,
			&row.WorstScore, &row.WorstReview); err != nil {
			return nil, fmt.Errorf("store: top de libros mejor evaluados: %w", err)
		}
		books = append(books, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: top de libros mejor evaluados: %w", err)
	}
	return books, nil
}

// ---------------------------------------------------------------------------
// 3. Top 50 libros más vendidos, con las ventas del autor y el flag de si el
//    libro estuvo en el top 5 de ventas de su año de publicación.
// ---------------------------------------------------------------------------

type TopSellingBookRow struct {
	BookID          int64
	BookName        string
	AuthorID        int64
	AuthorName      string
	BookSales       int
	AuthorSales     int
	PublicationYear int
	InTop5OfYear    bool
}

// topSellingBooksSQL responde la parte ambigua del enunciado con esta
// interpretación, también anotada en el DEVLOG: un libro estuvo en el top 5 de
// su año de publicación si, rankeando **todos** los libros por sus ventas de
// ese año calendario, quedó entre los 5 primeros. Es decir que compite contra
// todos los libros que vendieron ese año, no solo contra los publicados ese año.
//
// Se usa RANK() y no ROW_NUMBER() para que un empate en ventas ocupe la misma
// posición: si tres libros empatan en el primer puesto, los tres están en el
// top 5.
//
// author_totals se calcula aparte por la misma razón que en la tabla de
// autores: sumar las ventas del autor en el mismo JOIN multiplicaría los
// totales por la cantidad de filas de ventas.
const topSellingBooksSQL = `
WITH yearly_rank AS (
  SELECT book_id, year, sales,
         RANK() OVER (PARTITION BY year ORDER BY sales DESC) AS rk
  FROM sales_by_year
),
author_totals AS (
  SELECT author_id, COALESCE(SUM(number_of_sales), 0) AS author_sales
  FROM books
  GROUP BY author_id
)
SELECT b.id, b.name, a.id, a.name,
       b.number_of_sales,
       COALESCE(at.author_sales, 0),
       CAST(strftime('%Y', b.publication_date) AS INTEGER) AS publication_year,
       EXISTS (
         SELECT 1 FROM yearly_rank yr
         WHERE yr.book_id = b.id
           AND yr.year = CAST(strftime('%Y', b.publication_date) AS INTEGER)
           AND yr.rk <= 5
       ) AS in_top5_publication_year
FROM books b
JOIN authors a ON a.id = b.author_id
LEFT JOIN author_totals at ON at.author_id = b.author_id
ORDER BY b.number_of_sales DESC, b.id
LIMIT ?`

// TopSelling devuelve los limit libros más vendidos de todos los tiempos.
func (s *BookStore) TopSelling(ctx context.Context, limit int) ([]TopSellingBookRow, error) {
	rows, err := s.db.QueryContext(ctx, topSellingBooksSQL, limit)
	if err != nil {
		return nil, fmt.Errorf("store: top de libros más vendidos: %w", err)
	}
	defer rows.Close()

	var books []TopSellingBookRow
	for rows.Next() {
		var row TopSellingBookRow
		if err := rows.Scan(&row.BookID, &row.BookName, &row.AuthorID, &row.AuthorName,
			&row.BookSales, &row.AuthorSales, &row.PublicationYear,
			&row.InTop5OfYear); err != nil {
			return nil, fmt.Errorf("store: top de libros más vendidos: %w", err)
		}
		books = append(books, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: top de libros más vendidos: %w", err)
	}
	return books, nil
}
