package controllers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"xashloger/internal/adapters/http/models"
	"xashloger/internal/adapters/http/web"
)

type AdminController struct {
	model    *models.AdminModel
	renderer web.Renderer
}

func NewAdminController(model *models.AdminModel, renderer web.Renderer) *AdminController {
	return &AdminController{model: model, renderer: renderer}
}

func (c *AdminController) AdminPage(w http.ResponseWriter, r *http.Request) {
	data, err := c.model.BuildData(r.URL.Query().Get("period"))
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	c.renderer.Render(w, "layout", "admin", web.PageData{
		Title: "Admin",
		Data:  data,
	})
}

func (c *AdminController) AdminAPI(w http.ResponseWriter, r *http.Request) {
	data, err := c.model.BuildData(r.URL.Query().Get("period"))
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func (c *AdminController) TrackPlayer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad JSON", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.Name) == "" {
		http.Error(w, "Missing name", http.StatusBadRequest)
		return
	}

	if err := c.model.Track(req.Name); err != nil {
		log.Printf("Track error: %v", err)
		http.Error(w, fmt.Sprintf("Track error: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (c *AdminController) UntrackPlayer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad JSON", http.StatusBadRequest)
		return
	}

	if err := c.model.Untrack(req.Name); err != nil {
		log.Printf("Untrack error: %v", err)
		http.Error(w, fmt.Sprintf("Untrack error: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (c *AdminController) KickPlayer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name   string `json:"name"`
		Server string `json:"server"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad JSON", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.Name) == "" {
		http.Error(w, "Missing name", http.StatusBadRequest)
		return
	}

	if err := c.model.KickPlayer(req.Server, req.Name); err != nil {
		http.Error(w, "RCON error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (c *AdminController) AutoKickPlayerAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad JSON", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.Name) == "" {
		http.Error(w, "Missing name", http.StatusBadRequest)
		return
	}

	if err := c.model.AutoKickAdd(req.Name); err != nil {
		log.Printf("AutoKick add error: %v", err)
		http.Error(w, fmt.Sprintf("AutoKick add error: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (c *AdminController) AutoKickPlayerRemove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad JSON", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.Name) == "" {
		http.Error(w, "Missing name", http.StatusBadRequest)
		return
	}

	if err := c.model.AutoKickRemove(req.Name); err != nil {
		log.Printf("AutoKick remove error: %v", err)
		http.Error(w, fmt.Sprintf("AutoKick remove error: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (c *AdminController) FilterAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Pattern string `json:"pattern"`
		Type    string `json:"type"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad JSON", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.Pattern) == "" {
		http.Error(w, "Missing pattern", http.StatusBadRequest)
		return
	}
	if req.Type != "name" && req.Type != "message" {
		http.Error(w, "Invalid type", http.StatusBadRequest)
		return
	}

	if err := c.model.AddFilter(req.Pattern, req.Type); err != nil {
		log.Printf("Filter add error: %v", err)
		http.Error(w, fmt.Sprintf("Filter add error: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (c *AdminController) FilterRemove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID string `json:"id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad JSON", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.ID) == "" {
		http.Error(w, "Missing id", http.StatusBadRequest)
		return
	}

	if err := c.model.RemoveFilter(req.ID); err != nil {
		log.Printf("Filter remove error: %v", err)
		http.Error(w, fmt.Sprintf("Filter remove error: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
