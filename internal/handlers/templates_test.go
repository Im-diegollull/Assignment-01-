package handlers

import (
	"bytes"
	"testing"
)

// Las plantillas se ejecutan recién en el request, así que un error de campo
// mal escrito no lo detecta ni el compilador ni go vet. Este test las renderiza
// todas contra datos representativos.
func TestAuthorStatsSeRenderiza(t *testing.T) {
	templates, err := parseTemplates()
	if err != nil {
		t.Fatalf("parseTemplates: %v", err)
	}

	ts, ok := templates["authors_stats.html"]
	if !ok {
		t.Fatal("falta authors_stats.html")
	}

	page := authorStatsPage{
		Rows:    nil,
		Headers: authorStatsHeaders(nil),
		Filters: authorStatsFilters{MinBooks: "tres"},
		Ignored: []string{"libros (mínimo)"},
	}

	var buf bytes.Buffer
	if err := ts.ExecuteTemplate(&buf, "base", page); err != nil {
		t.Fatalf("ejecutar authors_stats.html: %v", err)
	}
}
