package models

import (
	"strings"
	"time"

	"xashloger/internal/adapters/http/modules"
	"xashloger/internal/adapters/http/web"
	"xashloger/internal/core/domain"

	"gorm.io/gorm"
)

type PlayersModel struct {
	db *gorm.DB
}

func NewPlayersModel(db *gorm.DB) *PlayersModel {
	return &PlayersModel{db: db}
}

func (m *PlayersModel) BuildPageData(page int, search, dateFilter string) (web.PageData, error) {
	if dateFilter == "all" || dateFilter == "" {
		return m.playersFromPlayersTable(page, search, dateFilter)
	}

	return m.playersFromEvents(page, search, dateFilter)
}

func (m *PlayersModel) playersFromPlayersTable(page int, search, dateFilter string) (web.PageData, error) {
	// базовый подзапрос с ГЛОБАЛЬНЫМ ранком
	ranked := m.db.
		Model(&domain.Player{}).
		Select(`
			id,
			name,
			frags,
			deaths,
			last_visited,
			ROW_NUMBER() OVER (
				ORDER BY frags DESC, deaths ASC
			) AS rank
		`)

	// subquery
	query := m.db.Table("(?) AS ranked", ranked)

	// search ПОСЛЕ ранжирования
	if search != "" {
		query = query.Where(
			"LOWER(name) LIKE ?",
			"%"+strings.ToLower(search)+"%",
		)
	}

	// total с учётом search
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return web.PageData{}, err
	}

	// paginator
	paginator := modules.NewPaginator(int(total), page, 50)

	type row struct {
		ID          string
		Rank        int
		Name        string
		Frags       int
		Deaths      int
		LastVisited time.Time
	}

	var rows []row
	if err := query.
		Limit(paginator.PageSize).
		Offset(paginator.Offset()).
		Scan(&rows).Error; err != nil {
		return web.PageData{}, err
	}

	// view models
	views := make([]domain.PlayerView, 0, len(rows))
	for _, r := range rows {
		kd := 0.0
		if r.Deaths > 0 {
			kd = float64(r.Frags) / float64(r.Deaths)
		} else if r.Frags > 0 {
			kd = 999.99
		}

		views = append(views, domain.PlayerView{
			ID:             r.ID,
			Rank:           r.Rank,
			Name:           r.Name,
			Frags:          r.Frags,
			Deaths:         r.Deaths,
			KD:             kd,
			LastVisited:    r.LastVisited,
			LastVisitedFmt: r.LastVisited.Format("02.01.2006 15:04"),
		})
	}

	return web.PageData{
		Title:      "Players",
		Data:       views,
		Page:       paginator.Page,
		PageSize:   paginator.PageSize,
		Total:      paginator.Total,
		TotalPages: paginator.TotalPages,
		Paginator:  paginator,
		Params: map[string]string{
			"search": search,
			"date":   dateFilter,
		},
	}, nil
}

func (m *PlayersModel) playersFromEvents(page int, search, dateFilter string) (web.PageData, error) {
	now := time.Now()
	var from, to time.Time

	switch dateFilter {
	case "today":
		from = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		to = now
	case "yesterday":
		to = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		from = to.AddDate(0, 0, -1)
	case "last_7days":
		from = now.AddDate(0, 0, -7)
		to = now
	default:
		from = time.Time{}
		to = now
	}

	type row struct {
		Name   string
		Frags  int
		Deaths int
		Rank   int
	}

	args := []interface{}{from, to, from, to, from, to}
	searchLower := strings.ToLower(search)

	cte := `
	WITH aggregated AS (
		SELECT
			name,
			SUM(frags) AS frags,
			SUM(deaths) AS deaths
		FROM (
			SELECT player AS name, COUNT(*) AS frags, 0 AS deaths
			FROM events
			WHERE type='killed' AND timestamp >= ? AND timestamp < ?
			GROUP BY player

			UNION ALL

			SELECT target AS name, 0 AS frags, COUNT(*) AS deaths
			FROM events
			WHERE type='killed' AND timestamp >= ? AND timestamp < ?
			GROUP BY target

			UNION ALL

			SELECT player AS name, 0 AS frags, COUNT(*) AS deaths
			FROM events
			WHERE type='suicide' AND timestamp >= ? AND timestamp < ?
			GROUP BY player
		) t
		GROUP BY name
	),
	ranked AS (
		SELECT
			name,
			frags,
			deaths,
			ROW_NUMBER() OVER (ORDER BY frags DESC, deaths ASC) AS rank
		FROM aggregated
	)
	`

	where := ""
	var countArgs []interface{}
	if searchLower != "" {
		where = "WHERE LOWER(name) LIKE ?"
		countArgs = append(args, "%"+searchLower+"%")
	} else {
		countArgs = args
	}

	countSQL := cte + "SELECT COUNT(1) FROM ranked " + where
	var total int64
	if err := m.db.Raw(countSQL, countArgs...).Scan(&total).Error; err != nil {
		return web.PageData{}, err
	}

	paginator := modules.NewPaginator(int(total), page, 50)

	querySQL := cte + `
	SELECT name, frags, deaths, rank
	FROM ranked
	` + where + `
	ORDER BY rank
	LIMIT ? OFFSET ?
	`

	queryArgs := append([]interface{}{}, countArgs...)
	queryArgs = append(queryArgs, paginator.PageSize, paginator.Offset())

	var rows []row
	if err := m.db.Raw(querySQL, queryArgs...).Scan(&rows).Error; err != nil {
		return web.PageData{}, err
	}

	views := make([]domain.PlayerView, 0, len(rows))
	for _, r := range rows {
		kd := 0.0
		if r.Deaths > 0 {
			kd = float64(r.Frags) / float64(r.Deaths)
		} else if r.Frags > 0 {
			kd = 999.99
		}

		views = append(views, domain.PlayerView{
			Name:   r.Name,
			Frags:  r.Frags,
			Deaths: r.Deaths,
			KD:     kd,
			Rank:   r.Rank,
		})
	}

	return web.PageData{
		Title:      "Players",
		Data:       views,
		Page:       paginator.Page,
		PageSize:   paginator.PageSize,
		Total:      paginator.Total,
		TotalPages: paginator.TotalPages,
		Paginator:  paginator,
		Params: map[string]string{
			"search": search,
			"date":   dateFilter,
		},
	}, nil
}
