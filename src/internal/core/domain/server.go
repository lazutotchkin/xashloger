package domain

import "time"

type ServerStat struct {
	ID            uint
	ServerName    string
	CPUUsage      CPUStat
	MemoryUsedMB  int64
	MemoryTotalMB int64
	Players       int
	MaxPlayers    int
	Online        bool
	UpdatedAt     time.Time
}

type OnlinePlayer struct {
	ID       uint
	Name     string
	IP       string
	ServerID uint
}

type MonitoredPlayer struct {
	ID   uint
	Name string
}

type CPUStat struct {
	User      uint64
	Nice      uint64
	System    uint64
	Interrupt uint64
	Idle      uint64
}

type CPUPoint struct {
	Time  string  `json:"time"`
	Value float64 `json:"value"`
}

type AdminPlayer struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	IP     string `json:"ip"`
	Kills  int    `json:"kills"`
	Deaths int    `json:"deaths"`
}
