package ports

import (
	"time"

	"xashloger/internal/core/domain"
)

// HLDSRepository defines persistence operations used by the HLDS usecase.
type HLDSRepository interface {
	IsAutokickPlayer(name string) (bool, error)
	IsIPBlackListed(ipPort string) (bool, error)
	IsFiltered(filterType, value string) (bool, error)
	IsTrackListIp(ipPort string) (bool, error)
	GetUserForTracking(name string) (*domain.Player, error)

	UpsertPlayer(name string, deltaFrags, deltaDeaths int) error
	UpdateLastVisited(player string, ts time.Time) error
	SaveEvent(e *domain.Event) error
	CleanupEvents(ttlHours int) error
}

// AdminRepository defines admin-side operations used by HTTP handlers.
type AdminRepository interface {
	TrackPlayer(name string) error
	UntrackPlayer(name string) error
	AddAutoKick(name string) error
	RemoveAutoKick(name string) error
	AddFilter(pattern, filterType string) error
	RemoveFilter(id string) error
	ListFilters() ([]domain.FilterList, error)

	AddIPToTrackList(ip string, expiresAt *time.Time) error
	RemoveIPFromTrackList(ip string) error
	GetIPTrackList() ([]domain.IPTrackList, error)

	AddIPToBlackList(ip string, expiresAt *time.Time) error
	RemoveIPFromBlackList(ip string) error
	GetIPBlackList() ([]domain.IPBlackList, error)

	TrackedPlayers() ([]domain.Player, error)
	GetAutoKickPlayers() ([]domain.Player, error)
}

// Mailer sends notifications (email).
type Mailer interface {
	Send(subject, body string, logMessage string) error
}

// RCONClient executes commands against HLDS.
type RCONClient interface {
	Exec(cmd string) (string, error)
}

// RCONFactory creates clients for specific servers.
type RCONFactory interface {
	New(addr, password string) RCONClient
}
