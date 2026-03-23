package models

import (
	"strings"

	"xashloger/internal/adapters/http/modules"
	"xashloger/internal/adapters/http/web"
	"xashloger/internal/core/domain"

	"gorm.io/gorm"
)

type EventsModel struct {
	db *gorm.DB
}

func NewEventsModel(db *gorm.DB) *EventsModel {
	return &EventsModel{db: db}
}

func (m *EventsModel) BuildPageData(filters EventsFilters) (web.PageData, error) {
	// base query
	base := m.db.Model(&domain.Event{})

	if filters.Search != "" {
		p := "%" + strings.ToLower(filters.Search) + "%"
		base = base.Where(
			m.db.
				Where("LOWER(player) LIKE ?", p).
				Or("LOWER(target) LIKE ?", p).
				Or("LOWER(source_ip) LIKE ?", p),
		)
	}

	if filters.EventType != "" {
		base = base.Where("type = ?", filters.EventType)
	}

	if filters.Server != "" {
		base = base.Where("server_ip = ?", filters.Server)
	}

	if filters.From != "" {
		base = base.Where("timestamp >= ?", filters.From)
	}

	if filters.To != "" {
		base = base.Where("timestamp <= ?", filters.To)
	}

	// total AFTER filters
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return web.PageData{}, err
	}

	// paginator
	paginator := modules.NewPaginator(int(total), filters.Page, 100)

	// data
	var events []domain.Event
	if err := base.
		Order("timestamp DESC").
		Limit(paginator.PageSize).
		Offset(paginator.Offset()).
		Find(&events).Error; err != nil {
		return web.PageData{}, err
	}

	// dictionaries for filters
	var eventTypes []string
	m.db.
		Model(&domain.Event{}).
		Distinct("type").
		Order("type").
		Pluck("type", &eventTypes)

	var servers []string
	m.db.
		Model(&domain.Event{}).
		Distinct("server_ip").
		Order("server_ip").
		Pluck("server_ip", &servers)

	return web.PageData{
		Title:      "Events",
		Data:       events,
		Page:       paginator.Page,
		PageSize:   paginator.PageSize,
		Total:      paginator.Total,
		TotalPages: paginator.TotalPages,
		Paginator:  paginator,
		EventTypes: eventTypes,
		Servers:    servers,
		Params: map[string]string{
			"search": filters.Search,
			"type":   filters.EventType,
			"server": filters.Server,
			"from":   filters.From,
			"to":     filters.To,
		},
	}, nil
}

type EventsFilters struct {
	Page      int
	Search    string
	EventType string
	Server    string
	From      string
	To        string
}
