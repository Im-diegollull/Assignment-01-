package handlers

import (
	"fmt"
	"html/template"
	"io/fs"
	"path/filepath"

	"bookreviews/web"
)

// parseTemplates arma un conjunto de plantillas independiente por cada página
// de web/templates/pages. Cada conjunto incluye el layout base y todos los
// parciales, de modo que dos páginas puedan definir el bloque "content" sin
// pisarse entre sí (lo que ocurriría si se parseara todo en un único
// template.Template).
func parseTemplates() (map[string]*template.Template, error) {
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
		ts, err := template.New(name).Funcs(templateFuncs).ParseFS(
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
}
