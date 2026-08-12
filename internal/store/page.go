package store

const DefaultPageSize = 20

type Page struct {
	Number int // 1-based
	Size   int
	Total  int // total de filas sin paginar; lo completa el store
}

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
