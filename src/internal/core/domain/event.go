package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Event struct {
	ID        string    `gorm:"primaryKey"`
	Timestamp time.Time `gorm:"not null;index"`
	Type      string    `gorm:"index"`
	Player    string    `gorm:"index"`
	PlayerID  string
	SteamID   string
	Team      string
	Target    string `gorm:"index"`
	Weapon    string
	Message   string
	Raw       string
	ServerIP  string    `gorm:"index"`
	SourceIP  string    `gorm:"index"`
	TTL       time.Time `gorm:"not null;index"`
}

func (e *Event) BeforeCreate(tx *gorm.DB) (err error) {
	e.ID = uuid.NewString()
	return
}
