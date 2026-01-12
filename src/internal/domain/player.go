package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Player struct {
	ID          string    `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"unique;not null" json:"name"`
	Frags       int       `json:"frags"`
	Deaths      int       `json:"deaths"`
	LastVisited time.Time `json:"lastVisited"`
	Track       bool      `gorm:"default:false" json:"track"`
	AutoKick    bool      `gorm:"default:false" json:"autoKick"`
}

type StatusPlayer struct {
	UserID  string
	Name    string
	SteamID string
	Time    string
	Ping    string
	IP      string
}

func (p *Player) BeforeCreate(tx *gorm.DB) (err error) {
	p.ID = uuid.NewString()
	return
}

type PlayerView struct {
	ID             string
	Rank           int
	Name           string
	Frags          int
	Deaths         int
	KD             float64
	LastVisited    time.Time
	LastVisitedFmt string
}
