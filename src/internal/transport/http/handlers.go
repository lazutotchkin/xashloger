package http

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"xashloger/internal/config"
	"xashloger/internal/domain"
	"xashloger/internal/repository"
	"xashloger/internal/transport/http/modules"
	"xashloger/internal/transport/udp/rcon"

	"gorm.io/gorm"
)

//go:embed templates/*.html
var templatesFS embed.FS

var (
	lastCPU     domain.CPUStat
	lastCPULock sync.Mutex

	cpuHistoryByDay = make(map[string][]domain.CPUPoint)
	cpuLock         sync.Mutex

	lastCPUDate string // YYYY-MM-DD
)

type ServerStatus struct {
	Online int    `json:"online"`
	Max    int    `json:"max"`
	Map    string `json:"map"`
}

const maxCPUPoints = 24 * 60

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

type HTTPServer struct {
	cfg       *config.Config
	db        *gorm.DB
	templates *template.Template
}

type AdminPageData struct {
	CPUUsage        float64                         `json:"cpuUsage"`
	CPUHistory      []domain.CPUPoint               `json:"cpuHistory"`
	MemoryUsedMB    int64                           `json:"memoryUsedMB"`
	MemoryFreeMB    int64                           `json:"memoryFreeMB"`
	MemoryTotalMB   int64                           `json:"memoryTotalMB"`
	ServerStatus    map[string]ServerStatus         `json:"serverStatus"`
	PlayersByServer map[string][]domain.AdminPlayer `json:"playersByServer"`
	TrackedPlayers  []domain.Player                 `json:"trackedPlayers"`
	AutokickPlayers []domain.Player                 `json:"autokickPlayers"`
}

func NewHTTPServer(repo *repository.HLDSRepository, cfg *config.Config) *HTTPServer {
	tmpl := template.Must(
		template.New("").
			Funcs(template.FuncMap{
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
			}).
			ParseFS(templatesFS, "templates/*.html"),
	)

	return &HTTPServer{
		db:        repo.DB(),
		templates: tmpl,
		cfg:       cfg,
	}
}

func (s *HTTPServer) renderPage(
	w http.ResponseWriter,
	layout string,
	page string,
	data PageData,
) {
	tmpl := template.Must(
		template.New("layout").
			Funcs(template.FuncMap{
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
			}).
			ParseFS(
				templatesFS,
				"templates/layout.html",
				"templates/"+page+".html",
				"templates/paginator.html",
			),
	)

	if err := tmpl.ExecuteTemplate(w, "layout", data); err != nil {
		http.Error(w, err.Error(), 500)
	}
}

func (s *HTTPServer) PlayersPage(w http.ResponseWriter, r *http.Request) {
	// page
	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}

	// search
	search := strings.TrimSpace(r.URL.Query().Get("search"))

	// базовый подзапрос с ГЛОБАЛЬНЫМ ранком
	ranked := s.db.
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
	query := s.db.Table("(?) AS ranked", ranked)

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
		http.Error(w, err.Error(), 500)
		return
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
		http.Error(w, err.Error(), 500)
		return
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

	// render
	s.renderPage(
		w,
		"layout",
		"players",
		PageData{
			Title:      "Players",
			Data:       views,
			Page:       paginator.Page,
			PageSize:   paginator.PageSize,
			Total:      paginator.Total,
			TotalPages: paginator.TotalPages,
			Paginator:  paginator,
			Params: map[string]string{
				"search": search,
			},
		},
	)
}

func (s *HTTPServer) EventsPage(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	// page
	page := 1
	if p := q.Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}

	// filters
	player := strings.TrimSpace(q.Get("player"))
	eventType := strings.TrimSpace(q.Get("type"))
	server := strings.TrimSpace(q.Get("server"))
	from := strings.TrimSpace(q.Get("from"))
	to := strings.TrimSpace(q.Get("to"))

	// base query
	base := s.db.Model(&domain.Event{})

	if player != "" {
		p := "%" + strings.ToLower(player) + "%"
		base = base.Where(
			s.db.
				Where("LOWER(player) LIKE ?", p).
				Or("LOWER(target) LIKE ?", p),
		)
	}

	if eventType != "" {
		base = base.Where("type = ?", eventType)
	}

	if server != "" {
		base = base.Where("server_ip = ?", server)
	}

	if from != "" {
		base = base.Where("timestamp >= ?", from)
	}

	if to != "" {
		base = base.Where("timestamp <= ?", to)
	}

	// total AFTER filters
	var total int64
	if err := base.Count(&total).Error; err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	// paginator
	paginator := modules.NewPaginator(int(total), page, 100)

	// data
	var events []domain.Event
	if err := base.
		Order("timestamp DESC").
		Limit(paginator.PageSize).
		Offset(paginator.Offset()).
		Find(&events).Error; err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	// dictionaries for filters
	var eventTypes []string
	s.db.
		Model(&domain.Event{}).
		Distinct("type").
		Order("type").
		Pluck("type", &eventTypes)

	var servers []string
	s.db.
		Model(&domain.Event{}).
		Distinct("server_ip").
		Order("server_ip").
		Pluck("server_ip", &servers)

	// render
	s.renderPage(
		w,
		"layout",
		"events",
		PageData{
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
				"player": player,
				"type":   eventType,
				"server": server,
				"from":   from,
				"to":     to,
			},
		},
	)
}

func (s *HTTPServer) AdminPage(w http.ResponseWriter, r *http.Request) {
	data, err := s.collectAdminData(r)
	if err != nil {
		http.Error(w, "Internal error", 500)
		return
	}

	s.renderPage(w, "layout", "admin", PageData{
		Title: "Admin",
		Data:  data,
	})
}

func (s *HTTPServer) AdminAPI(w http.ResponseWriter, r *http.Request) {
	data, err := s.collectAdminData(r)
	if err != nil {
		http.Error(w, "Internal error", 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func getMemoryUsageMB() (used int64, total int64, err error) {
	get := func(name string) (int64, error) {
		out, err := exec.Command("sysctl", "-n", name).Output()
		if err != nil {
			return 0, err
		}
		return strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	}

	totalBytes, err := get("hw.physmem")
	if err != nil {
		return
	}

	pageSize, err := get("hw.pagesize")
	if err != nil {
		return
	}

	freePages, err := get("vm.stats.vm.v_free_count")
	if err != nil {
		return
	}

	inactivePages, err := get("vm.stats.vm.v_inactive_count")
	if err != nil {
		return
	}

	cachePages, err := get("vm.stats.vm.v_cache_count")
	if err != nil {
		return
	}

	freeBytes :=
		(freePages + inactivePages + cachePages) * pageSize

	total = totalBytes / 1024 / 1024
	used = (totalBytes - freeBytes) / 1024 / 1024

	return
}

func cpuUsage(prev, cur domain.CPUStat) float64 {
	idle := cur.Idle - prev.Idle
	total :=
		(cur.User - prev.User) +
			(cur.Nice - prev.Nice) +
			(cur.System - prev.System) +
			(cur.Interrupt - prev.Interrupt) +
			idle

	if total == 0 {
		return 0
	}

	return 100 * float64(total-idle) / float64(total)
}

func getCPUStat() (domain.CPUStat, error) {
	out, err := exec.Command("sysctl", "-n", "kern.cp_time").Output()
	if err != nil {
		return domain.CPUStat{}, err
	}

	fields := strings.Fields(string(out))
	if len(fields) < 5 {
		return domain.CPUStat{}, fmt.Errorf("unexpected cp_time format")
	}

	parse := func(s string) uint64 {
		v, _ := strconv.ParseUint(s, 10, 64)
		return v
	}

	return domain.CPUStat{
		User:      parse(fields[0]),
		Nice:      parse(fields[1]),
		System:    parse(fields[2]),
		Interrupt: parse(fields[3]),
		Idle:      parse(fields[4]),
	}, nil
}

func StartCPUCollector() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		day := now.Format("2006-01-02")

		cur, err := getCPUStat()
		if err != nil {
			continue
		}

		lastCPULock.Lock()

		var cpu float64

		if lastCPU.User == 0 {
			lastCPU = cur
			lastCPULock.Unlock()
			continue
		}

		if lastCPU.User != 0 {
			cpu = cpuUsage(lastCPU, cur)
		}
		lastCPU = cur
		lastCPULock.Unlock()

		point := domain.CPUPoint{
			Time:  now.Format("2006-01-02 15:04"),
			Value: cpu,
		}

		cpuLock.Lock()

		cpuHistoryByDay[day] = append(cpuHistoryByDay[day], point)

		// ограничение: 7 дней
		if len(cpuHistoryByDay) > 7 {
			cutoff := now.AddDate(0, 0, -7).Format("2006-01-02")
			delete(cpuHistoryByDay, cutoff)
		}

		cpuLock.Unlock()
	}
}

func getServerStatus(addr string) (online int, err error) {
	host, port, _ := strings.Cut(addr, ":")

	conn, err := net.DialTimeout("udp", host+":"+port, time.Second)
	if err != nil {
		return
	}
	defer conn.Close()

	req := []byte{0xff, 0xff, 0xff, 0xff, 0x69, 0x6e, 0x66, 0x6f, 0x20, 0x34, 0x39}
	conn.Write(req)

	buf := make([]byte, 512)
	n, err := conn.Read(buf)
	if err != nil {
		return
	}

	parts := strings.Split(string(buf[:n]), "\\")
	if len(parts) < 13 {
		err = fmt.Errorf("bad response")
		return
	}

	fmt.Sscanf(parts[12], "%d", &online)
	return
}

func getPlayers(addr string, rconPass string) ([]domain.AdminPlayer, error) {
	RCONClient := rcon.NewRCON(addr, rconPass)

	out, err := RCONClient.Exec("status")
	if err != nil {
		return nil, err
	}

	return rcon.ParseRCONPlayers(out), nil
}

func (s *HTTPServer) TrackPlayer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad JSON", 400)
		return
	}

	if req.Name == "" {
		http.Error(w, "Missing name", 400)
		return
	}

	repo := repository.NewHLDSRepository(s.db)

	if err := repo.TrackPlayer(req.Name); err != nil {
		log.Printf("Track error: %v", err)
		http.Error(w, fmt.Sprintf("AutoKick add error: %v", err), 500)
		return
	}

	w.WriteHeader(http.StatusOK)

}

func (s *HTTPServer) UntrackPlayer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad JSON", 400)
		return
	}

	repo := repository.NewHLDSRepository(s.db)

	if err := repo.UntrackPlayer(req.Name); err != nil {
		log.Printf("Untrack error: %v", err)
		http.Error(w, fmt.Sprintf("AutoKick add error: %v", err), 500)
		return
	}

	w.WriteHeader(http.StatusOK)

}

func (s *HTTPServer) KickPlayer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name   string `json:"name"`
		Server string `json:"server"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad JSON", 400)
		return
	}

	if req.Name == "" {
		http.Error(w, "Missing name", 400)
		return
	}

	rconClient := rcon.NewRCON(req.Server, "secret9")
	cmd := fmt.Sprintf("kick \"%s\"", strings.TrimSpace(req.Name))
	if _, err := rconClient.Exec(cmd); err != nil {
		http.Error(w, "RCON error", 500)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *HTTPServer) AutoKickPlayerAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad JSON", 400)
		return
	}

	if req.Name == "" {
		http.Error(w, "Missing name", 400)
		return
	}

	repo := repository.NewHLDSRepository(s.db)

	if err := repo.AddAutoKick(req.Name); err != nil {
		log.Printf("AutoKick add error: %v", err)
		http.Error(w, fmt.Sprintf("AutoKick add error: %v", err), 500)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *HTTPServer) AutoKickPlayerRemove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad JSON", 400)
		return
	}

	if req.Name == "" {
		http.Error(w, "Missing name", 400)
		return
	}

	repo := repository.NewHLDSRepository(s.db)

	if err := repo.RemoveAutoKick(req.Name); err != nil {
		log.Printf("AutoKick remove error: %v", err)
		http.Error(w, fmt.Sprintf("AutoKick add error: %v", err), 500)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *HTTPServer) collectAdminData(r *http.Request) (AdminPageData, error) {
	// servers := []string{
	// 	"127.0.0.1:27015",
	// 	"127.0.0.1:27016",
	// 	"127.0.0.1:27017",
	// }

	// serverMaps := map[string]string{
	// 	"127.0.0.1:27015": "stalkyard",
	// 	"127.0.0.1:27016": "crossfire",
	// 	"127.0.0.1:27017": "bounce",
	// }

	serverMaps := make(map[string]string)
	for _, srv := range s.cfg.TrackingServers {
		if srv.Enabled {
			serverMaps[net.JoinHostPort(srv.Addr, strconv.Itoa(srv.Port))] = srv.Name
		}
	}

	serverStatus := map[string]ServerStatus{}
	var wg sync.WaitGroup
	var mu sync.Mutex

	for addr, mapName := range serverMaps {
		// mapName := mapName
		wg.Add(1)

		go func() {
			defer wg.Done()

			on, err := getServerStatus(addr)
			if err != nil {
				return
			}

			mu.Lock()
			serverStatus[addr] = ServerStatus{
				Online: on,
				Max:    16,
				Map:    strings.Title(mapName),
			}
			mu.Unlock()
		}()
	}

	// CPU
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "today"
	}

	history := getCPUHistory(period)

	cpu := 0.0
	if len(history) > 0 {
		sum := 0.0
		for _, item := range history {
			sum += item.Value
		}
		cpu = sum / float64(len(history))
	}

	// Mem
	memUsed, memTotal, _ := getMemoryUsageMB()

	// Players
	playersByServer := make(map[string][]domain.AdminPlayer)

	for addr, _ := range serverMaps {

		rconPass := ""
		for _, srv := range s.cfg.TrackingServers {
			if net.JoinHostPort(srv.Addr, strconv.Itoa(srv.Port)) == addr {
				rconPass = srv.RCONPassword
				break
			}
		}

		if rconPass == "" {
			continue
		}
		players, err := getPlayers(addr, rconPass)
		if err != nil {
			continue
		}

		adminPlayers := make([]domain.AdminPlayer, 0, len(players))
		for _, p := range players {
			adminPlayers = append(adminPlayers, domain.AdminPlayer{
				ID:   p.ID,
				Name: p.Name,
				IP:   p.IP,
			})
		}
		playersByServer[addr] = adminPlayers
	}

	// Tracked
	repo := repository.NewHLDSRepository(s.db)
	tracked, _ := repo.TrackedPlayers()

	autokick, _ := repo.GetAutoKickPlayers()
	return AdminPageData{
		CPUUsage:        cpu,
		CPUHistory:      history,
		MemoryUsedMB:    memUsed,
		MemoryFreeMB:    memTotal - memUsed,
		MemoryTotalMB:   memTotal,
		ServerStatus:    serverStatus,
		PlayersByServer: playersByServer,
		TrackedPlayers:  tracked,
		AutokickPlayers: autokick,
	}, nil
}

func getCPUHistory(period string) []domain.CPUPoint {
	cpuLock.Lock()
	defer cpuLock.Unlock()

	today := time.Now()
	result := []domain.CPUPoint{}

	switch period {
	case "today":
		result = cpuHistoryByDay[today.Format("2006-01-02")]

	case "yesterday":
		result = cpuHistoryByDay[today.AddDate(0, 0, -1).Format("2006-01-02")]

	case "week":
		for i := 6; i >= 0; i-- {
			day := today.AddDate(0, 0, -i).Format("2006-01-02")
			result = append(result, cpuHistoryByDay[day]...)
		}
	}

	return result
}
