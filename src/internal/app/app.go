package app

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"syscall"
	"xashloger/internal/config"
	"xashloger/internal/db"
	"xashloger/internal/logger"
	"xashloger/internal/repository"
	"xashloger/internal/service"
	"xashloger/internal/transport/http"
	"xashloger/internal/transport/udp"
	"xashloger/internal/utils/mailto"

	"github.com/sirupsen/logrus"
)

type App struct {
	cfg *config.Config
}

func New(cfg *config.Config) *App {
	return &App{cfg: cfg}
}

func (app *App) Run() {

	defer func() {
		if r := recover(); r != nil {
			logrus.Errorf("APPLICATION PANIC: %v", r)
			debug.PrintStack()
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	)
	defer stop()

	go func() {
		<-ctx.Done()
		logrus.Warn("Application shutting down by signal")
		stop()
	}()

	// LOGGER
	logDir := app.cfg.Logs.LogDir
	if logDir == "" {
		logDir = "logs"
	}

	retain := app.cfg.Logs.RetainDays
	if retain <= 0 {
		retain = 7
	}

	if err := logger.Init(app.cfg, logDir, retain); err != nil {
		logrus.Errorf("failed to initialize logger: %v", err)
	}
	defer logger.Close()

	// DB
	dbPath := ""
	if app.cfg.Flags.Production {
		exePath, err := os.Executable()
		if err != nil {
			logrus.Errorf("failed to get executable path: %v", err)
		}
		os.MkdirAll(filepath.Dir(filepath.Join(filepath.Dir(exePath), app.cfg.Database.Path)), 0755)
		dbPath = filepath.Join(filepath.Dir(exePath), app.cfg.Database.Path)
	} else {
		wd, err := os.Getwd()
		if err != nil {
			logrus.Errorf("failed to get working dir: %v", err)
		}
		os.MkdirAll(filepath.Dir(filepath.Join(wd, app.cfg.Database.Path)), 0755)
		dbPath = filepath.Join(wd, app.cfg.Database.Path)
	}

	logrus.Infof("db path: %s", dbPath)

	dbConn, err := db.Open(dbPath)
	if err != nil {
		logrus.Errorf("SQLite open error: %v", err)
		return
	}

	repo := repository.NewHLDSRepository(dbConn)
	if err := repo.Init(); err != nil {
		logrus.Errorf("repository init error: %v", err)
	}

	mailer := mailto.New(app.cfg)

	logic := service.NewHLDSService(repo, mailer)

	// UDP
	udpAddr := app.cfg.Server.Addr + ":" + strconv.Itoa(app.cfg.Server.Port)
	udpServer := udp.New(udpAddr, logic)

	// HTTP
	webAddr := app.cfg.WebServer.Addr + ":" + strconv.Itoa(app.cfg.WebServer.Port)
	httpServer := http.NewHTTPServer(repo, app.cfg)

	// RUN SERVERS IN GOROUTINES
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logrus.Errorf("UDP goroutine panic: %v\n%s", r, debug.Stack())
			}
		}()

		logrus.Infof("Starting UDP server on %s", udpAddr)
		if err := udpServer.Run(); err != nil {
			logrus.Fatalf("UDP server error: %v", err)
			return
		}
	}()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				logrus.Errorf("HTTP goroutine panic: %v\n%s", r, debug.Stack())
			}
		}()

		logrus.Infof("Starting HTTP server on %s", webAddr)
		if err := httpServer.Run(webAddr); err != nil {
			logrus.Fatalf("HTTP server error: %v", err)
			return
		}
	}()

	select {}
}
