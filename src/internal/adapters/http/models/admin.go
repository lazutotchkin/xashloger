package models

import (
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"xashloger/internal/adapters/rcon"
	"xashloger/internal/core/domain"
	"xashloger/internal/core/ports"
)

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
	Filters         []domain.FilterList             `json:"filters"`
}

type ServerStatus struct {
	Online int    `json:"online"`
	Max    int    `json:"max"`
	Map    string `json:"map"`
}

type ServerInfo struct {
	Name string
	RCON string
}

type AdminModel struct {
	adminRepo ports.AdminRepository
	servers   map[string]ServerInfo
}

func NewAdminModel(adminRepo ports.AdminRepository, servers map[string]ServerInfo) *AdminModel {
	return &AdminModel{adminRepo: adminRepo, servers: servers}
}

func (m *AdminModel) BuildData(period string) (AdminPageData, error) {
	serverStatus := make(map[string]ServerStatus)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for addr, info := range m.servers {
		wg.Add(1)
		go func(addr string, info ServerInfo) {
			defer wg.Done()

			on, err := getServerStatus(addr)
			if err != nil {
				log.Printf("Get server status error (%s): %v", addr, err)
				on = 0
			}

			mu.Lock()
			serverStatus[addr] = ServerStatus{
				Online: on,
				Max:    16,
				Map:    strings.Title(info.Name),
			}
			mu.Unlock()
		}(addr, info)
	}

	wg.Wait()

	// CPU
	if period == "" {
		period = "today"
	}

	history := GetCPUHistory(period)

	cpu := 0.0
	if len(history) > 0 {
		sum := 0.0
		for _, item := range history {
			sum += item.Value
		}
		cpu = sum / float64(len(history))
	}

	// Mem
	memUsed, memTotal, _ := GetMemoryUsageMB()

	// Players
	playersByServer := make(map[string][]domain.AdminPlayer)
	var pwg sync.WaitGroup
	var pmu sync.Mutex

	for addr, info := range m.servers {
		if info.RCON == "" {
			continue
		}

		pwg.Add(1)
		go func(addr, rconPass string) {
			defer pwg.Done()

			players, err := getPlayers(addr, rconPass)
			if err != nil {
				return
			}

			adminPlayers := make([]domain.AdminPlayer, 0, len(players))
			for _, p := range players {
				adminPlayers = append(adminPlayers, domain.AdminPlayer{
					ID:     p.ID,
					Name:   p.Name,
					IP:     p.IP,
					Kills:  p.Kills,
					Deaths: p.Deaths,
				})
			}

			pmu.Lock()
			playersByServer[addr] = adminPlayers
			pmu.Unlock()
		}(addr, info.RCON)
	}

	pwg.Wait()

	// Tracked
	tracked, _ := m.adminRepo.TrackedPlayers()

	autokick, _ := m.adminRepo.GetAutoKickPlayers()

	ipTrackList, _ := m.adminRepo.GetIPTrackList()
	for _, ip := range ipTrackList {
		tracked = append(tracked, domain.Player{
			Name: ip.IP,
		})
	}

	ipBlackList, _ := m.adminRepo.GetIPBlackList()
	for _, ip := range ipBlackList {
		autokick = append(autokick, domain.Player{
			Name: ip.IP,
		})
	}

	filters, _ := m.adminRepo.ListFilters()

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
		Filters:         filters,
	}, nil
}

func (m *AdminModel) Track(name string) error {
	if isIP(name) {
		return m.adminRepo.AddIPToTrackList(name, nil)
	}
	return m.adminRepo.TrackPlayer(name)
}

func (m *AdminModel) Untrack(name string) error {
	if isIP(name) {
		return m.adminRepo.RemoveIPFromTrackList(name)
	}
	return m.adminRepo.UntrackPlayer(name)
}

func (m *AdminModel) AutoKickAdd(name string) error {
	if isIP(name) {
		return m.adminRepo.AddIPToBlackList(name, nil)
	}
	return m.adminRepo.AddAutoKick(name)
}

func (m *AdminModel) AutoKickRemove(name string) error {
	if isIP(name) {
		return m.adminRepo.RemoveIPFromBlackList(name)
	}
	return m.adminRepo.RemoveAutoKick(name)
}

func (m *AdminModel) KickPlayer(server, name string) error {
	rconClient := rcon.NewRCON(server, "secret9")
	cmd := fmt.Sprintf("kick \"%s\"", strings.TrimSpace(name))
	_, err := rconClient.Exec(cmd)
	return err
}

func (m *AdminModel) AddFilter(pattern, filterType string) error {
	return m.adminRepo.AddFilter(pattern, filterType)
}

func (m *AdminModel) RemoveFilter(id string) error {
	return m.adminRepo.RemoveFilter(id)
}

func isIP(s string) bool {
	return net.ParseIP(s) != nil
}

func getPlayers(addr, pass string) ([]domain.AdminPlayer, error) {
	client := rcon.NewRCON(addr, pass)
	out, err := client.Exec("status")
	if err != nil {
		return nil, err
	}
	return rcon.ParseRCONPlayers(out), nil
}

func getServerStatus(addr string) (online int, err error) {
	var conn net.Conn
	conn, err = net.DialTimeout("udp", addr, 1*time.Second)

	if err != nil {
		return 0, fmt.Errorf("failed to connect to %s: %w", addr, err)
	}
	defer conn.Close()

	req := []byte{0xff, 0xff, 0xff, 0xff, 0x69, 0x6e, 0x66, 0x6f, 0x20, 0x34, 0x39}
	conn.Write(req)

	buf := make([]byte, 1400)
	conn.SetReadDeadline(time.Now().Add(1 * time.Second))
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
