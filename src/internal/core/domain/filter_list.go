package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type FilterList struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	Pattern   string    `gorm:"column:pattern;not null" json:"pattern"`
	Type      string    `gorm:"column:type;not null" json:"type"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
}

func (FilterList) TableName() string {
	return "filter_list"
}

func (fl *FilterList) BeforeCreate(tx *gorm.DB) (err error) {
	fl.ID = uuid.NewString()
	return
}
