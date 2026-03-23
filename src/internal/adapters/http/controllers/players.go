package controllers

import (
	"net/http"
	"strings"

	"xashloger/internal/adapters/http/models"
	"xashloger/internal/adapters/http/web"
)

type PlayersController struct {
	model    *models.PlayersModel
	renderer web.Renderer
}

func NewPlayersController(model *models.PlayersModel, renderer web.Renderer) *PlayersController {
	return &PlayersController{model: model, renderer: renderer}
}

func (c *PlayersController) PlayersPage(w http.ResponseWriter, r *http.Request) {
	page := parsePageParam(r)
	search := strings.TrimSpace(r.URL.Query().Get("search"))
	dateFilter := strings.TrimSpace(r.URL.Query().Get("date"))

	data, err := c.model.BuildPageData(page, search, dateFilter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	c.renderer.Render(w, "layout", "players", data)
}
