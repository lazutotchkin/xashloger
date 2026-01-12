package parser

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"xashloger/internal/domain"
)

var (
	sayRe = regexp.MustCompile(
		`(\d{2}/\d{2}/\d{4} - \d{2}:\d{2}:\d{2}): "(.+?)<(\d+)><([^>]*)><([^>]*)>" say "(.*)"`,
	)

	connRe = regexp.MustCompile(
		`(\d{2}/\d{2}/\d{4} - \d{2}:\d{2}:\d{2}): "(.+?)<(\d+)><([^>]*)><([^>]*)>" connected, address "([\d\.]+):(\d+)"`,
	)

	disconnRe = regexp.MustCompile(
		`(\d{2}/\d{2}/\d{4} - \d{2}:\d{2}:\d{2}): "(.+?)<(\d+)><([^>]*)><([^>]*)>" disconnected`,
	)

	killedRe = regexp.MustCompile(
		`(\d{2}/\d{2}/\d{4} - \d{2}:\d{2}:\d{2}): "(.+?)<(\d+)><([^>]*)><([^>]*)>" killed "(.+?)<(\d+)><([^>]*)><([^>]*)>" with "(.*?)"`,
	)

	suicideRe = regexp.MustCompile(
		`(\d{2}/\d{2}/\d{4} - \d{2}:\d{2}:\d{2}): "(.+?)<(\d+)><([^>]*)><([^>]*)>" committed suicide with "(.*?)"`,
	)

	nameChangeRe = regexp.MustCompile(
		`(\d{2}/\d{2}/\d{4} - \d{2}:\d{2}:\d{2}): "(.+?)<(\d+)><([^>]*)><([^>]*)>" changed name to "(.*)"`,
	)
)

func ParseHLDSLog(line string) (*domain.LogEntry, error) {
	line = strings.TrimPrefix(line, "L ")
	line = strings.TrimSpace(line)

	var entry domain.LogEntry
	entry.Raw = line
	timestampStr := ""

	switch {
	case sayRe.MatchString(line):
		m := sayRe.FindStringSubmatch(line)
		timestampStr = m[1]
		entry.Type = "say"
		entry.Player = m[2]
		entry.PlayerID = m[3]
		entry.SteamID = m[4]
		entry.Team = m[5]
		entry.Message = m[6]

	case connRe.MatchString(line):
		m := connRe.FindStringSubmatch(line)
		timestampStr = m[1]
		entry.Type = "connected"
		entry.Player = m[2]
		entry.PlayerID = m[3]
		entry.SteamID = m[4]
		entry.Team = m[5]
		entry.SourceIP = m[6] + ":" + m[7]

	case disconnRe.MatchString(line):
		m := disconnRe.FindStringSubmatch(line)
		timestampStr = m[1]
		entry.Type = "disconnected"
		entry.Player = m[2]
		entry.PlayerID = m[3]
		entry.SteamID = m[4]
		entry.Team = m[5]

	case killedRe.MatchString(line):
		m := killedRe.FindStringSubmatch(line)
		timestampStr = m[1]
		entry.Type = "killed"
		entry.Player = m[2]
		entry.PlayerID = m[3]
		entry.SteamID = m[4]
		entry.Team = m[5]
		entry.Target = m[6]
		entry.Weapon = m[10]

	case suicideRe.MatchString(line):
		m := suicideRe.FindStringSubmatch(line)
		timestampStr = m[1]
		entry.Type = "suicide"
		entry.Player = m[2]
		entry.PlayerID = m[3]
		entry.SteamID = m[4]
		entry.Team = m[5]
		entry.Weapon = m[6]
	case nameChangeRe.MatchString(line):
		m := nameChangeRe.FindStringSubmatch(line)
		timestampStr = m[1]
		entry.Type = "name_change"
		entry.Player = m[2]
		entry.PlayerID = m[3]
		entry.SteamID = m[4]
		entry.Team = m[5]
		entry.Target = m[6]

	default:
		entry.Type = "unknown"
		return &entry, fmt.Errorf("unrecognized log type")
	}

	ts, err := time.Parse("01/02/2006 - 15:04:05", timestampStr)
	if err != nil {
		return nil, fmt.Errorf("cannot parse timestamp: %v", err)
	}
	entry.Timestamp = ts
	return &entry, nil
}
