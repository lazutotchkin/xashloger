package repository

import (
	"fmt"
	"strings"
	"time"
	"xashloger/internal/domain"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type HLDSRepository struct {
	db *gorm.DB
}

func NewHLDSRepository(db *gorm.DB) *HLDSRepository {
	return &HLDSRepository{db: db}
}

func (r *HLDSRepository) Init() error {
	return r.db.AutoMigrate(&domain.Player{}, &domain.Event{})
}

func (r *HLDSRepository) SaveEvent(e *domain.Event) error {
	if err := r.db.Create(e).Error; err != nil {
		logrus.Errorf("Failed to save event: %v", err)
		return err
	}
	logrus.Infof("Event saved: %+v", e)
	return nil
}

func (r *HLDSRepository) UpsertPlayer(name string, deltaFrags, deltaDeaths int) error {
	player := domain.Player{
		Name:   name,
		Frags:  deltaFrags,
		Deaths: deltaDeaths,
	}

	err := r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "name"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"frags":  gorm.Expr("frags + ?", deltaFrags),
			"deaths": gorm.Expr("deaths + ?", deltaDeaths),
		}),
	}).Create(&player).Error

	if err != nil {
		logrus.Errorf("Failed to upsert player %s: %v", name, err)
		return err
	}

	logrus.Infof("Player upserted: %s (+%d frags, +%d deaths)", name, deltaFrags, deltaDeaths)
	return nil
}

func (r *HLDSRepository) CleanupEvents(ttlHours int) error {
	if err := r.db.Table("events").Where("ttl < ?", time.Now()).Delete(&domain.Event{}).Error; err != nil {
		logrus.Errorf("Failed to cleanup events: %v", err)
		return err
	}
	logrus.Infof("Cleanup events older than %d hours", ttlHours)
	return nil
}

func (r *HLDSRepository) DB() *gorm.DB {
	return r.db
}

func (r *HLDSRepository) UpdateLastVisited(player string, ts time.Time) error {
	return r.db.Model(&domain.Player{}).
		Where("name = ?", player).
		Update("last_visited", ts).
		Error
}

func (r *HLDSRepository) TrackPlayer(name string) error {
	exists, err := r.CheckPlayerExists(name)
	if err != nil {
		return err
	}

	if !exists {
		return fmt.Errorf("player %s does not exist in database", name)
	}

	return r.db.Model(&domain.Player{}).
		Where("name = ?", strings.TrimSpace(name)).
		Update("track", true).
		Error
}

func (r *HLDSRepository) UntrackPlayer(name string) error {
	exists, err := r.CheckPlayerExists(name)
	if err != nil {
		return err
	}

	if !exists {
		return fmt.Errorf("player %s does not exist in database", name)
	}

	return r.db.Model(&domain.Player{}).
		Where("name = ?", name).
		Update("track", false).
		Error
}

func (r *HLDSRepository) CheckPlayerExists(name string) (bool, error) {
	var count int64
	err := r.db.Model(&domain.Player{}).
		Where("name = ?", name).
		Count(&count).Error

	return count > 0, err
}

func (r *HLDSRepository) TrackedPlayers() ([]domain.Player, error) {
	var users []domain.Player
	return users, r.db.
		Where("track = ?", true).
		Order("name desc").
		Find(&users).Error
}

func (r *HLDSRepository) GetUserForTracking(name string) (*domain.Player, error) {
	var player domain.Player
	err := r.db.
		Where("name = ? AND track = ?", name, true).
		First(&player).Error

	if err != nil {
		return nil, err
	}

	return &player, nil
}

func (r *HLDSRepository) GetAutoKickPlayers() ([]domain.Player, error) {
	var players []domain.Player
	err := r.db.
		Where("auto_kick = ?", true).
		Find(&players).Error

	if err != nil {
		return nil, err
	}

	return players, nil
}

func (r *HLDSRepository) RemoveAutoKick(name string) error {
	exists, err := r.CheckPlayerExists(name)
	if err != nil {
		return err
	}

	if !exists {
		return fmt.Errorf("player %s does not exist in database", name)
	}

	return r.db.Model(&domain.Player{}).
		Where("name = ? AND auto_kick = ?", name, true).
		Update("auto_kick", false).
		Error
}

func (r *HLDSRepository) AddAutoKick(name string) error {
	exists, err := r.CheckPlayerExists(name)
	if err != nil {
		return err
	}

	if !exists {
		return fmt.Errorf("player %s does not exist in database", name)
	}

	return r.db.Model(&domain.Player{}).
		Where("name = ? AND auto_kick = ?", name, false).
		Update("auto_kick", true).
		Error
}

func (r *HLDSRepository) IsAutokickPlayer(name string) (bool, error) {
	var count int64
	err := r.db.
		Model(&domain.Player{}).
		Where("name = ? AND auto_kick = ?", name, true).
		Count(&count).Error

	return count > 0, err
}
