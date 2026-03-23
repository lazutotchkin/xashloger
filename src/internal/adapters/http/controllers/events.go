package controllers

import (
	"net/http"
	"strconv"
	"strings"

	"xashloger/internal/adapters/http/models"
	"xashloger/internal/adapters/http/web"
)

type EventsController struct {
	model    *models.EventsModel
	renderer web.Renderer
}

func NewEventsController(model *models.EventsModel, renderer web.Renderer) *EventsController {
	return &EventsController{model: model, renderer: renderer}
}

func (c *EventsController) EventsPage(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	page := 1
	if p := q.Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}

	filters := models.EventsFilters{
		Page:      page,
		Search:    strings.TrimSpace(q.Get("search")),
		EventType: strings.TrimSpace(q.Get("type")),
		Server:    strings.TrimSpace(q.Get("server")),
		From:      strings.TrimSpace(q.Get("from")),
		To:        strings.TrimSpace(q.Get("to")),
	}

	data, err := c.model.BuildPageData(filters)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	c.renderer.Render(w, "layout", "events", data)
}
