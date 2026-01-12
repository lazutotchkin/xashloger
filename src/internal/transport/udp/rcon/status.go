package rcon

import (
	"strings"
	"xashloger/internal/domain"
)

func ParseRCONPlayers(out string) []domain.AdminPlayer {
	var players []domain.AdminPlayer

	lines := strings.Split(out, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		if line == "" ||
			strings.HasPrefix(line, "map:") ||
			strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		id := fields[0]
		ip := fields[len(fields)-1]

		uaEnd := strings.Index(line, ")")
		if uaEnd == -1 {
			continue
		}

		// Всё после useragent
		rest := strings.TrimSpace(line[uaEnd+1:])

		if strings.HasSuffix(rest, ip) {
			rest = strings.TrimSpace(strings.TrimSuffix(rest, ip))
		}

		name := rest

		players = append(players, domain.AdminPlayer{
			ID:   id,
			Name: name,
			IP:   ip,
		})
	}

	return players
}
