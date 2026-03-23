package rcon

import (
	"strconv"
	"strings"
	"xashloger/internal/core/domain"
)

func ParseRCONPlayers(out string) []domain.AdminPlayer {
	var players []domain.AdminPlayer

	lines := strings.Split(out, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "log ") || strings.HasPrefix(line, "9No challenge") {
			continue
		}
		if strings.HasPrefix(line, "map:") || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}

		// ID игрока
		id := fields[0]

		// score берём как kills
		kills := parseInt(fields[1])

		// HLDS не отдаёт deaths в status, оставляем 0
		deaths := 0

		// IP игрока — последний элемент
		ip := fields[len(fields)-1]

		// Имя игрока — всё после useragent и до IP
		uaEnd := strings.Index(line, ")")
		if uaEnd == -1 {
			continue
		}

		name := strings.TrimSpace(line[uaEnd+1:])
		if strings.HasSuffix(name, ip) {
			name = strings.TrimSpace(strings.TrimSuffix(name, ip))
		}

		players = append(players, domain.AdminPlayer{
			ID:     id,
			Name:   name,
			IP:     ip,
			Kills:  kills,
			Deaths: deaths,
		})
	}

	return players
}

func parseInt(s string) int {
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return v
}
