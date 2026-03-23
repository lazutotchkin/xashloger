package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type IPBlackList struct {
	ID        string    `gorm:"primaryKey"`
	IP        string    `gorm:"uniqueIndex;not null"`
	AddedAt   time.Time `gorm:"not null"`
	ExpiresAt *time.Time
}

func (IPBlackList) TableName() string {
	return "ip_black_list"
}

func (ip *IPBlackList) BeforeCreate(tx *gorm.DB) (err error) {
	ip.ID = uuid.NewString()
	if ip.AddedAt.IsZero() {
		ip.AddedAt = time.Now()
	}
	return
}
