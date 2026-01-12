package db

import (
	"database/sql"
	"fmt"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	_ "modernc.org/sqlite"
)

func Open(path string) (*gorm.DB, error) {
	// DSN для modernc.org/sqlite
	dsn := fmt.Sprintf("file:%s?_foreign_keys=1&_busy_timeout=30000&_journal_mode=WAL", path)

	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite db: %w", err)
	}

	pragmas := []string{

		"PRAGMA synchronous = NORMAL;",
		"PRAGMA temp_store = MEMORY;",
		"PRAGMA cache_size = -100000;",
	}

	for _, p := range pragmas {
		if _, err := sqlDB.Exec(p); err != nil {
			return nil, fmt.Errorf("pragma failed (%s): %w", p, err)
		}
	}

	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetConnMaxLifetime(0)

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping sqlite db: %w", err)
	}

	gormDB, err := gorm.Open(sqlite.Dialector{Conn: sqlDB}, &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to open gorm db: %w", err)
	}

	return gormDB, nil
}
