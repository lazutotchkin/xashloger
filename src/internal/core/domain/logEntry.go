package domain

import "time"

type LogEntry struct {
	Timestamp time.Time
	Type      string // say, connected, disconnected, killed ...
	Player    string
	PlayerID  string
	SteamID   string
	Team      string
	Target    string // кого убил
	TargetID  string
	Weapon    string // чем убил
	Message   string // say
	Raw       string
	SourceIP  string
	ServerIP  string
}
