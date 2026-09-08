package store

import (
	"sort"
	"strings"
)


func ApplyAuthorFilter(rows []AuthorStatsRow, filter AuthorFilter) []AuthorStatsRow {
	name := strings.ToLower(strings.TrimSpace(filter.NameLike))

	out := make([]AuthorStatsRow, 0, len(rows))
	for _, row := range rows {
		if name != "" && !strings.Contains(strings.ToLower(row.Name), name) {
			continue
		}
		if filter.MinBooks != nil && row.BooksCount < *filter.MinBooks {
			continue
		}
		if filter.MaxBooks != nil && row.BooksCount > *filter.MaxBooks {
			continue
		}
		if filter.MinSales != nil && row.TotalSales < *filter.MinSales {
			continue
		}
		if filter.MaxSales != nil && row.TotalSales > *filter.MaxSales {
			continue
		}
		if filter.MinScore != nil && (!row.AvgScore.Valid || row.AvgScore.Float64 < *filter.MinScore) {
			continue
		}
		if filter.MaxScore != nil && (!row.AvgScore.Valid || row.AvgScore.Float64 > *filter.MaxScore) {
			continue
		}
		out = append(out, row)
	}

	sortBy := filter.SortBy
	if _, known := authorSortColumns[sortBy]; !known {
		sortBy = "name"
	}
	desc := strings.EqualFold(filter.Dir, "desc")

	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if cmp := compareAuthorStats(a, b, sortBy); cmp != 0 {
			if desc {
				return cmp > 0
			}
			return cmp < 0
		}
		return a.ID < b.ID
	})
	return out
}

func compareAuthorStats(a, b AuthorStatsRow, sortBy string) int {
	switch sortBy {
	case "books":
		return a.BooksCount - b.BooksCount
	case "total_sales":
		return a.TotalSales - b.TotalSales
	case "avg_score":
		return compareNullFloat(a.AvgScore.Valid, a.AvgScore.Float64, b.AvgScore.Valid, b.AvgScore.Float64)
	default:
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	}
}

// NULLs se tratan como más chicos que cualquier número, como en SQLite.
func compareNullFloat(aValid bool, a float64, bValid bool, b float64) int {
	switch {
	case !aValid && !bValid:
		return 0
	case !aValid:
		return -1
	case !bValid:
		return 1
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}
