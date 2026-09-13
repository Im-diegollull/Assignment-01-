package handlers

import (
	"fmt"
	"html/template"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"

	"bookreviews/web"
)

func parseTemplates() (map[string]*template.Template, error) {
	return parseTemplatesWithDebug("off", "SQLite")
}

func parseTemplatesWithDebug(cacheName, searchName string) (map[string]*template.Template, error) {
	pages, err := fs.Glob(web.Files, "templates/pages/*.html")
	if err != nil {
		return nil, fmt.Errorf("handlers: buscar páginas: %w", err)
	}
	if len(pages) == 0 {
		return nil, fmt.Errorf("handlers: no se encontraron plantillas en templates/pages")
	}

	sets := make(map[string]*template.Template, len(pages))
	for _, page := range pages {
		name := filepath.Base(page)
		ts, err := template.New(name).Funcs(templateFuncs).Funcs(template.FuncMap{
			"debugCache":  func() string { return cacheName },
			"debugSearch": func() string { return searchName },
		}).ParseFS(
			web.Files,
			"templates/base.html",
			"templates/partials/*.html",
			page,
		)
		if err != nil {
			return nil, fmt.Errorf("handlers: parsear %s: %w", name, err)
		}
		sets[name] = ts
	}
	return sets, nil
}

// templateFuncs son las funciones disponibles dentro de las plantillas.
// Se mantienen al mínimo: el formateo pertenece a la vista, la lógica al store.
var templateFuncs = template.FuncMap{
	// yesNo convierte un booleano en texto legible para las tablas.
	"yesNo": func(b bool) string {
		if b {
			return "Sí"
		}
		return "No"
	},

	// thousands separa los miles con puntos. Las ventas llegan a seis cifras y
	// sin separador las columnas son imposibles de comparar de un vistazo.
	"thousands": thousands,

	// add existe solo para numerar las filas de un ranking a partir del índice
	// de range, que empieza en 0.
	"add": func(a, b int) int { return a + b },
}

func thousands(n int) string {
	digits := strconv.Itoa(n)
	sign := ""
	if strings.HasPrefix(digits, "-") {
		sign, digits = "-", digits[1:]
	}

	var out strings.Builder
	for i, digit := range digits {
		// Un punto cada tres dígitos, contando desde la derecha.
		if i > 0 && (len(digits)-i)%3 == 0 {
			out.WriteByte('.')
		}
		out.WriteRune(digit)
	}
	return sign + out.String()
}
