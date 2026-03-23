package web

import (
	"embed"
	"html/template"
	"net/http"

	"xashloger/internal/adapters/http/modules"
)

//go:embed templates/*.html
var templatesFS embed.FS

type PageData struct {
	Title      string
	Data       interface{}
	Page       int
	PageSize   int
	Total      int
	TotalPages int
	Paginator  *modules.Paginator
	Servers    []string
	EventTypes []string

	Params map[string]string
}

type Renderer interface {
	Render(w http.ResponseWriter, layout, page string, data PageData)
}

type renderer struct {
	templates map[string]*template.Template
}

func NewRenderer() Renderer {
	return &renderer{templates: buildTemplates()}
}

func (v *renderer) Render(w http.ResponseWriter, layout, page string, data PageData) {
	tmpl := v.templates[page]
	if tmpl == nil {
		http.Error(w, "Template not found", http.StatusInternalServerError)
		return
	}

	if err := tmpl.ExecuteTemplate(w, layout, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
		"mul": func(a, b int) int { return a * b },
		"min": func(a, b int) int {
			if a < b {
				return a
			}
			return b
		},
		"max": func(a, b int) int {
			if a > b {
				return a
			}
			return b
		},
		"seq": func(from, to int) []int {
			s := []int{}
			for i := from; i <= to; i++ {
				s = append(s, i)
			}
			return s
		},
		"append": func(s []interface{}, v interface{}) []interface{} {
			return append(s, v)
		},
	}
}

func buildTemplates() map[string]*template.Template {
	pages := []string{"players", "events", "admin"}
	cache := make(map[string]*template.Template, len(pages))
	for _, page := range pages {
		tmpl := template.Must(
			template.New("layout").
				Funcs(templateFuncs()).
				ParseFS(
					templatesFS,
					"templates/layout.html",
					"templates/"+page+".html",
					"templates/paginator.html",
				),
		)
		cache[page] = tmpl
	}
	return cache
}
