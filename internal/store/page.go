package store

// DefaultPageSize es el tamaño de página de los listados del CRUD.
const DefaultPageSize = 20

// Page describe una porción de un listado largo. Los métodos los consumen
// directamente las plantillas para dibujar los controles de paginación.
type Page struct {
	Number int // 1-based
	Size   int
	Total  int // total de filas sin paginar; lo completa el store
}

// NewPage normaliza los valores que vienen de la query string: un ?page=-3 o un
// ?page=abc (que el handler convierte en 0) caen en la página 1.
func NewPage(number, size int) Page {
	if number < 1 {
		number = 1
	}
	if size < 1 {
		size = DefaultPageSize
	}
	return Page{Number: number, Size: size}
}

func (p Page) Offset() int {
	return (p.Number - 1) * p.Size
}

func (p Page) TotalPages() int {
	if p.Total == 0 {
		return 1
	}
	// División hacia arriba sin float: evita errores de redondeo.
	return (p.Total + p.Size - 1) / p.Size
}

func (p Page) HasPrev() bool { return p.Number > 1 }
func (p Page) HasNext() bool { return p.Number < p.TotalPages() }
func (p Page) PrevNumber() int {
	return p.Number - 1
}
func (p Page) NextNumber() int {
	return p.Number + 1
}

// From y To describen el rango mostrado ("21–40 de 300"). From devuelve 0
// cuando no hay filas, para que la plantilla pueda mostrar el estado vacío.
func (p Page) From() int {
	if p.Total == 0 {
		return 0
	}
	return p.Offset() + 1
}

func (p Page) To() int {
	if end := p.Offset() + p.Size; end < p.Total {
		return end
	}
	return p.Total
}
