package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Event struct {
	ID        string    `gorm:"primaryKey"`
	Timestamp time.Time `gorm:"not null"`
	Type      string
	Player    string
	PlayerID  string
	SteamID   string
	Team      string
	Target    string
	Weapon    string
	Message   string
	Raw       string
	ServerIP  string
	SourceIP  string
	TTL       time.Time `gorm:"not null"`
}

func (e *Event) BeforeCreate(tx *gorm.DB) (err error) {
	e.ID = uuid.NewString()
	return
}
