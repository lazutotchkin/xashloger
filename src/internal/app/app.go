package app

import (
	"context"
	"fmt"
	"io"
	nethttp "net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"syscall"
	"time"
	"xashloger/internal/adapters/http"
	mailto "xashloger/internal/adapters/mailer"
	"xashloger/internal/adapters/rcon"
	"xashloger/internal/adapters/repository"
	"xashloger/internal/adapters/udp"
	"xashloger/internal/core/usecase"
	"xashloger/internal/infra/config"
	"xashloger/internal/infra/db"
	"xashloger/internal/infra/logger"

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

	logrus.Infof("Start xashloger")

	// DB
	dbPath := app.cfg.Database.Path
	if dbPath == "" {
		logrus.Errorf("database path is empty in config")
		return
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		logrus.Errorf("failed to create db dir: %v", err)
		return
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
	repository.SetHangNotifier(func(err error, duration time.Duration) {
		subject := "SQLite hang detected"
		body := fmt.Sprintf("db: %s\nerror: %v\nduration: %s\n", dbPath, err, duration)
		if sendErr := mailer.SendError(subject, body); sendErr != nil {
			logrus.Warnf("failed to send db hang email: %v", sendErr)
		}
	})

	rconFactory := rcon.NewFactory()
	logic := usecase.NewHLDSService(repo, mailer, rconFactory)

	// UDP
	udpAddr := app.cfg.Server.Addr + ":" + strconv.Itoa(app.cfg.Server.Port)
	udpServer := udp.New(udpAddr, logic)

	// HTTP
	webAddr := app.cfg.WebServer.Addr + ":" + strconv.Itoa(app.cfg.WebServer.Port)
	httpServer := http.NewHTTPServer(repo.DB(), repo, app.cfg)

	// RUN SERVERS IN GOROUTINES
	go func() {
		runWithRestart(ctx, "UDP", 2*time.Second, func() error {
			logrus.Infof("Starting UDP server on %s", udpAddr)
			return udpServer.Run(ctx)
		})
	}()

	go func() {
		runWithRestart(ctx, "HTTP", 3*time.Second, func() error {
			logrus.Infof("Starting HTTP server on %s", webAddr)
			return httpServer.Run(ctx, webAddr)
		})
	}()

	go func() {
		logrus.Infof("Starting pprof on 127.0.0.1:6060")
		if err := nethttp.ListenAndServe("127.0.0.1:6060", nil); err != nil {
			logrus.Errorf("pprof server error: %v", err)
		}
	}()

	go logMemStats(ctx, 2*time.Minute)
	go saveHeapProfiles(ctx, 5*time.Minute, "/root/xashloger_v2/pprof")

	<-ctx.Done()
}

func runWithRestart(ctx context.Context, name string, delay time.Duration, fn func() error) {
	attempt := 0
	for {
		if ctx.Err() != nil {
			logrus.Infof("%s server stop requested: %v", name, ctx.Err())
			return
		}

		attempt++
		started := time.Now()
		logrus.Infof("%s server start attempt #%d", name, attempt)

		err := func() error {
			defer func() {
				if r := recover(); r != nil {
					logrus.Errorf("%s server panic: %v\n%s", name, r, debug.Stack())
				}
			}()
			return fn()
		}()

		uptime := time.Since(started)
		if err != nil {
			logrus.Errorf("%s server stopped with error after %s: %v", name, uptime, err)
		} else {
			logrus.Warnf("%s server stopped without error after %s", name, uptime)
		}

		if ctx.Err() != nil {
			logrus.Infof("%s server stop requested after exit: %v", name, ctx.Err())
			return
		}

		logrus.Warnf("%s server restart scheduled in %s", name, delay)
		time.Sleep(delay)
	}
}

func logMemStats(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			var ms runtime.MemStats
			runtime.ReadMemStats(&ms)
			logrus.Infof("memstats alloc=%dMB heap_alloc=%dMB heap_sys=%dMB heap_objects=%d num_gc=%d",
				ms.Alloc/1024/1024,
				ms.HeapAlloc/1024/1024,
				ms.HeapSys/1024/1024,
				ms.HeapObjects,
				ms.NumGC,
			)
		}
	}
}

func saveHeapProfiles(ctx context.Context, interval time.Duration, dir string) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		logrus.Errorf("pprof dir create error: %v", err)
		return
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ts := time.Now().Format("2006-01-02-15-04")
			path := filepath.Join(dir, ts+".heap")

			resp, err := nethttp.Get("http://127.0.0.1:6060/debug/pprof/heap")
			if err != nil {
				logrus.Errorf("pprof heap fetch error: %v", err)
				continue
			}
			func() {
				defer resp.Body.Close()
				if resp.StatusCode != 200 {
					logrus.Errorf("pprof heap status: %s", resp.Status)
					return
				}
				f, err := os.Create(path)
				if err != nil {
					logrus.Errorf("pprof heap file error: %v", err)
					return
				}
				defer f.Close()
				if _, err := io.Copy(f, resp.Body); err != nil {
					logrus.Errorf("pprof heap write error: %v", err)
					return
				}
				logrus.Infof("pprof heap saved: %s", path)
			}()
		}
	}
}
