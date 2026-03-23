package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"xashloger/internal/infra/config"

	"github.com/fatih/color"
	"github.com/sirupsen/logrus"
)

var mgr *manager
var closeOnce sync.Once

type manager struct {
	dir         string
	mu          sync.Mutex
	f           *os.File
	retention   int
	rotateTimer *time.Timer
	writer      io.Writer
	quit        chan struct{}
}

type customFormatter struct {
	Color bool
}

func (f *customFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	t := entry.Time.Format("2006-01-02 15:04:05")
	level := strings.ToUpper(entry.Level.String())
	msg := entry.Message

	if f.Color {
		switch entry.Level {
		case logrus.ErrorLevel, logrus.FatalLevel, logrus.PanicLevel:
			level = color.RedString(level)
		case logrus.WarnLevel:
			level = color.YellowString(level)
		case logrus.InfoLevel:
			level = color.GreenString(level)
		case logrus.DebugLevel, logrus.TraceLevel:
			level = color.CyanString(level)
		}
	}

	line := fmt.Sprintf("[%s] [%s] %s\n", t, level, msg)
	return []byte(line), nil
}

// Init initializes logging: daily file named YYYY-MM-DD.log and cleaner
func Init(cfg *config.Config, dir string, retentionDays int) error {
	if dir == "" {
		dir = "logs"
	}
	if retentionDays <= 0 {
		retentionDays = 7
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	m := &manager{dir: dir, retention: retentionDays, quit: make(chan struct{})}
	if err := m.openForToday(); err != nil {
		return err
	}
	// set logrus output to file
	// logrus.SetOutput(m.f)
	m.writer = io.MultiWriter(os.Stdout, m.f)
	logrus.SetOutput(m.writer)
	logrus.SetLevel(logrus.TraceLevel)
	logrus.SetFormatter(&customFormatter{
		Color: cfg.Logs.Color, // true / false
	})

	if cfg.Flags.Production {
		logrus.SetLevel(logrus.InfoLevel)
	} else {
		logrus.SetLevel(logrus.DebugLevel)
	}

	m.deleteOld()

	mgr = m
	go m.rotationLoop()
	go m.cleanerLoop()
	return nil
}

func (m *manager) openForToday() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.f != nil {
		m.f.Close()
	}
	name := time.Now().Format("2006-01-02") + ".log"
	path := filepath.Join(m.dir, name)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	m.f = f
	return nil
}

func (m *manager) rotationLoop() {
	for {
		// compute duration until next midnight
		now := time.Now()
		next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
		d := time.Until(next)
		timer := time.NewTimer(d)
		select {
		case <-m.quit:
			timer.Stop()
			return
		case <-timer.C:
			// rotate file
			m.openForToday()
			m.writer = io.MultiWriter(os.Stdout, m.f)
			logrus.SetOutput(m.writer)

		}
	}
}

func (m *manager) cleanerLoop() {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-m.quit:
			return
		case <-ticker.C:
			m.deleteOld()
		}
	}
}

func (m *manager) deleteOld() {
	files, err := os.ReadDir(m.dir)
	if err != nil {
		logrus.Errorf("failed to read log directory '%s': %v", m.dir, err)
		return
	}

	cutoff := time.Now().AddDate(0, 0, -m.retention).Format("2006-01-02")
	logrus.Infof("running log cleanup, retention: %d days, cutoff: %s", m.retention, cutoff)

	for _, fi := range files {
		if fi.IsDir() {
			continue
		}

		name := fi.Name()
		if !strings.HasSuffix(name, ".log") {
			continue
		}

		// name format: YYYY-MM-DD.log
		datePart := strings.TrimSuffix(name, ".log")

		if datePart < cutoff {
			fullPath := filepath.Join(m.dir, name)
			err := os.Remove(fullPath)
			if err != nil {
				logrus.Warnf("failed to delete old log file %s: %v", name, err)
				continue
			}
			logrus.Infof("deleted old log file: %s", name)
		} else {
			logrus.Debugf("keeping log file %s", name)
		}
	}
}

// Close stops background goroutines and closes file
func Close() error {
	if mgr == nil {
		return nil
	}

	var err error

	closeOnce.Do(func() {
		close(mgr.quit)

		mgr.mu.Lock()
		defer mgr.mu.Unlock()

		if mgr.f != nil {
			err = mgr.f.Close()
		}
	})

	return err
}
