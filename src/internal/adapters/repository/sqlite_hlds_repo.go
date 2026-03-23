package repository

import (
	"fmt"
	"strings"
	"sync"
	"time"
	"xashloger/internal/core/domain"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type HLDSRepository struct {
	db *gorm.DB
}

const (
	retryAttempts  = 5
	retryDelay     = 200 * time.Millisecond
	hangThreshold  = 10 * time.Second
	notifyCooldown = 10 * time.Minute
)

var (
	hangNotifierMu sync.Mutex
	hangNotifier   func(err error, duration time.Duration)
	lastNotify     time.Time
)

func NewHLDSRepository(db *gorm.DB) *HLDSRepository {
	return &HLDSRepository{db: db}
}

func SetHangNotifier(fn func(err error, duration time.Duration)) {
	hangNotifierMu.Lock()
	defer hangNotifierMu.Unlock()
	hangNotifier = fn
}

func (r *HLDSRepository) Init() error {
	return r.db.AutoMigrate(&domain.Player{},
		&domain.Event{},
		&domain.IPBlackList{},
		&domain.IPTrackList{},
		&domain.FilterList{},
	)
}

func (r *HLDSRepository) SaveEvent(e *domain.Event) error {
	return r.withRetry(func() error {
		if err := r.db.Create(e).Error; err != nil {
			logrus.Errorf("Failed to save event: %v", err)
			return err
		}
		logrus.Debugf("Event saved: %+v", e)
		return nil
	})
}

func (r *HLDSRepository) UpsertPlayer(name string, deltaFrags, deltaDeaths int) error {
	player := domain.Player{
		Name:   name,
		Frags:  deltaFrags,
		Deaths: deltaDeaths,
	}

	return r.withRetry(func() error {
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

		logrus.Debugf("Player upserted: %s (+%d frags, +%d deaths)", name, deltaFrags, deltaDeaths)
		return nil
	})
}

func (r *HLDSRepository) CleanupEvents(ttlHours int) error {
	return r.withRetry(func() error {
		if err := r.db.Table("events").Where("ttl < ?", time.Now()).Delete(&domain.Event{}).Error; err != nil {
			logrus.Errorf("Failed to cleanup events: %v", err)
			return err
		}
		logrus.Infof("Cleanup events older than %d hours", ttlHours)
		return nil
	})
}

func (r *HLDSRepository) DB() *gorm.DB {
	return r.db
}

func (r *HLDSRepository) UpdateLastVisited(player string, ts time.Time) error {
	return r.withRetry(func() error {
		return r.db.Model(&domain.Player{}).
			Where("name = ?", player).
			Update("last_visited", ts).
			Error
	})
}

func (r *HLDSRepository) TrackPlayer(name string) error {
	exists, err := r.CheckPlayerExists(name)
	if err != nil {
		return err
	}

	if !exists {
		return fmt.Errorf("player %s does not exist in database", name)
	}

	return r.withRetry(func() error {
		return r.db.Model(&domain.Player{}).
			Where("name = ?", strings.TrimSpace(name)).
			Update("track", true).
			Error
	})
}

func (r *HLDSRepository) UntrackPlayer(name string) error {
	exists, err := r.CheckPlayerExists(name)
	if err != nil {
		return err
	}

	if !exists {
		return fmt.Errorf("player %s does not exist in database", name)
	}

	return r.withRetry(func() error {
		return r.db.Model(&domain.Player{}).
			Where("name = ?", name).
			Update("track", false).
			Error
	})
}

func (r *HLDSRepository) CheckPlayerExists(name string) (bool, error) {
	var count int64
	err := r.withRetry(func() error {
		return r.db.Model(&domain.Player{}).
			Where("name = ?", name).
			Count(&count).Error
	})

	return count > 0, err
}

func (r *HLDSRepository) TrackedPlayers() ([]domain.Player, error) {
	var users []domain.Player
	err := r.withRetry(func() error {
		return r.db.
			Where("track = ?", true).
			Order("name desc").
			Find(&users).Error
	})
	return users, err
}

func (r *HLDSRepository) GetUserForTracking(name string) (*domain.Player, error) {
	var player domain.Player
	err := r.withRetry(func() error {
		return r.db.
			Where("name = ? AND track = ?", name, true).
			First(&player).Error
	})

	if err != nil {
		return nil, err
	}

	return &player, nil
}

func (r *HLDSRepository) GetAutoKickPlayers() ([]domain.Player, error) {
	var players []domain.Player
	err := r.withRetry(func() error {
		return r.db.
			Where("auto_kick = ?", true).
			Find(&players).Error
	})

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

	return r.withRetry(func() error {
		return r.db.Model(&domain.Player{}).
			Where("name = ? AND auto_kick = ?", name, true).
			Update("auto_kick", false).
			Error
	})
}

func (r *HLDSRepository) AddAutoKick(name string) error {
	exists, err := r.CheckPlayerExists(name)
	if err != nil {
		return err
	}

	if !exists {
		return fmt.Errorf("player %s does not exist in database", name)
	}

	return r.withRetry(func() error {
		return r.db.Model(&domain.Player{}).
			Where("name = ? AND auto_kick = ?", name, false).
			Update("auto_kick", true).
			Error
	})
}

func (r *HLDSRepository) AddFilter(pattern, filterType string) error {
	filter := domain.FilterList{
		Pattern: strings.TrimSpace(pattern),
		Type:    filterType,
	}
	return r.withRetry(func() error {
		return r.db.Create(&filter).Error
	})
}

func (r *HLDSRepository) RemoveFilter(id string) error {
	return r.withRetry(func() error {
		return r.db.Where("id = ?", id).Delete(&domain.FilterList{}).Error
	})
}

func (r *HLDSRepository) ListFilters() ([]domain.FilterList, error) {
	var filters []domain.FilterList
	err := r.withRetry(func() error {
		return r.db.Order("created_at DESC").Find(&filters).Error
	})
	return filters, err
}

func (r *HLDSRepository) IsAutokickPlayer(name string) (bool, error) {
	var count int64
	err := r.withRetry(func() error {
		return r.db.
			Model(&domain.Player{}).
			Where("name = ? AND auto_kick = ?", name, true).
			Count(&count).Error
	})

	return count > 0, err
}

func (r *HLDSRepository) IsFiltered(filterType, value string) (bool, error) {
	if strings.TrimSpace(value) == "" {
		return false, nil
	}
	var count int64
	err := r.withRetry(func() error {
		return r.db.
			Model(&domain.FilterList{}).
			Where("type = ? AND LOWER(?) LIKE '%' || LOWER(pattern) || '%'", filterType, value).
			Count(&count).Error
	})

	return count > 0, err
}

func (r *HLDSRepository) AddIPToBlackList(ip string, expiresAt *time.Time) error {
	entry := domain.IPBlackList{
		IP:        ip,
		ExpiresAt: expiresAt,
	}

	return r.withRetry(func() error {
		return r.db.Create(&entry).Error
	})
}

func (r *HLDSRepository) IsIPBlackListed(ipPort string) (bool, error) {
	var count int64
	ip, _, _ := strings.Cut(ipPort, ":")

	err := r.withRetry(func() error {
		return r.db.Model(&domain.IPBlackList{}).
			Where("ip = ?", ip).
			Count(&count).Error
	})

	return count > 0, err
}

func (r *HLDSRepository) GetIPBlackList() ([]domain.IPBlackList, error) {
	var list []domain.IPBlackList
	err := r.withRetry(func() error {
		return r.db.Find(&list).Error
	})
	return list, err
}

func (r *HLDSRepository) RemoveIPFromBlackList(ip string) error {
	return r.withRetry(func() error {
		return r.db.Where("ip = ?", ip).Delete(&domain.IPBlackList{}).Error
	})
}

func (r *HLDSRepository) CleanupExpiredIPs() error {
	return r.withRetry(func() error {
		return r.db.Where("expires_at IS NOT NULL AND expires_at <= ?", time.Now()).Delete(&domain.IPBlackList{}).Error
	})
}

// Добавить IP в список отслеживания
func (r *HLDSRepository) AddIPToTrackList(ip string, expiresAt *time.Time) error {
	record := domain.IPTrackList{
		IP:        ip,
		ExpiresAt: expiresAt,
	}

	err := r.withRetry(func() error {
		return r.db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "ip"}},
			DoUpdates: clause.Assignments(map[string]interface{}{"expires_at": expiresAt}),
		}).Create(&record).Error
	})

	if err != nil {
		logrus.Errorf("Failed to add IP %s to track list: %v", ip, err)
		return err
	}

	logrus.Infof("IP %s added to track list", ip)
	return nil
}

// Удалить IP из списка отслеживания
func (r *HLDSRepository) RemoveIPFromTrackList(ip string) error {
	if err := r.withRetry(func() error {
		return r.db.Where("ip = ?", ip).Delete(&domain.IPTrackList{}).Error
	}); err != nil {
		logrus.Errorf("Failed to remove IP %s from track list: %v", ip, err)
		return err
	}

	logrus.Infof("IP %s removed from track list", ip)
	return nil
}

// Получить все IP в списке отслеживания
func (r *HLDSRepository) GetIPTrackList() ([]domain.IPTrackList, error) {
	var ips []domain.IPTrackList
	if err := r.withRetry(func() error {
		return r.db.Find(&ips).Error
	}); err != nil {
		return nil, err
	}
	return ips, nil
}

func (r *HLDSRepository) IsTrackListIp(ipPort string) (bool, error) {
	var count int64
	ip, _, _ := strings.Cut(ipPort, ":")

	err := r.withRetry(func() error {
		return r.db.Model(&domain.IPTrackList{}).
			Where("ip = ?", ip).
			Count(&count).Error
	})

	return count > 0, err
}

func (r *HLDSRepository) withRetry(op func() error) error {
	start := time.Now()
	var lastErr error
	for attempt := 0; attempt < retryAttempts; attempt++ {
		if err := op(); err != nil {
			lastErr = err
			if !isBusyErr(err) {
				notifyHang(err, time.Since(start))
				return err
			}
			time.Sleep(retryDelay * time.Duration(attempt+1))
			continue
		}
		return nil
	}
	notifyHang(lastErr, time.Since(start))
	return lastErr
}

func notifyHang(err error, duration time.Duration) {
	if err == nil {
		return
	}
	if duration < hangThreshold && !isBusyErr(err) {
		return
	}
	hangNotifierMu.Lock()
	defer hangNotifierMu.Unlock()
	if hangNotifier == nil {
		return
	}
	if time.Since(lastNotify) < notifyCooldown {
		return
	}
	lastNotify = time.Now()
	hangNotifier(err, duration)
}

func isBusyErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "database is locked") ||
		strings.Contains(msg, "database is busy") ||
		strings.Contains(msg, "sqlite_busy")
}
