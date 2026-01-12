package modules

import "math"

type Paginator struct {
	Page       int // текущая страница
	PageSize   int // элементов на страницу
	Total      int // всего элементов
	TotalPages int // всего страниц
}

// Создание нового пагинатора
func NewPaginator(total, page, pageSize int) *Paginator {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))
	if page > totalPages && totalPages > 0 {
		page = totalPages
	}

	return &Paginator{
		Page:       page,
		PageSize:   pageSize,
		Total:      total,
		TotalPages: totalPages,
	}
}

func (p *Paginator) Offset() int {
	return (p.Page - 1) * p.PageSize
}

func (p *Paginator) Pages() []int {
	pages := make([]int, p.TotalPages)
	for i := range pages {
		pages[i] = i + 1
	}
	return pages
}
