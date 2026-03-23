package http

import (
	"context"
	"net/http"
	"time"
	"xashloger/internal/adapters/http/custom_middleware"
	"xashloger/internal/adapters/http/models"
	"xashloger/internal/adapters/http/web"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/sirupsen/logrus"
)

func (s *HTTPServer) Run(ctx context.Context, addr string) error {
	go models.StartCPUCollector()

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(s.logrusMiddleware(logrus.StandardLogger()))
	r.Use(middleware.Recoverer)
	// r.Use(httprate.LimitByIP(5, 1*time.Second))

	r.Handle("/static/*", http.StripPrefix("/static/", web.StaticHandler()))

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/players", http.StatusMovedPermanently)
	})

	if s.cfg.WebServer.IpFirewall.Enabled {
		ipFilter, err := custom_middleware.NewIPFilter(
			s.cfg.WebServer.IpFirewall.Allow,
		)
		if err != nil {
			return err
		}

		r.With(ipFilter.Middleware).
			Get("/events", s.eventsCtrl.EventsPage)
		r.With(ipFilter.Middleware).
			Get("/admin", s.adminCtrl.AdminPage)
		r.With(ipFilter.Middleware).
			Post("/admin/kick", s.adminCtrl.KickPlayer)
		r.With(ipFilter.Middleware).
			Post("/admin/untrack", s.adminCtrl.UntrackPlayer)
		r.With(ipFilter.Middleware).
			Post("/admin/track", s.adminCtrl.TrackPlayer)
		r.With(ipFilter.Middleware).
			Post("/admin/kick", s.adminCtrl.KickPlayer)

		r.With(ipFilter.Middleware).
			Post("/admin/autokick/add", s.adminCtrl.AutoKickPlayerAdd)
		r.With(ipFilter.Middleware).
			Post("/admin/autokick/remove", s.adminCtrl.AutoKickPlayerRemove)
		r.With(ipFilter.Middleware).
			Post("/admin/filter/add", s.adminCtrl.FilterAdd)
		r.With(ipFilter.Middleware).
			Post("/admin/filter/remove", s.adminCtrl.FilterRemove)

		r.With(ipFilter.Middleware).
			Get("/api/admin", s.adminCtrl.AdminAPI)
	} else {
		r.Get("/events", s.eventsCtrl.EventsPage)
	}

	r.Get("/players", s.playersCtrl.PlayersPage)

	server := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}

func (s *HTTPServer) logrusMiddleware(logger *logrus.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			duration := time.Since(start)

			logger.Infof("HTTP %s %s from %s - %d (%s)",
				r.Method,
				r.URL.Path,
				r.RemoteAddr,
				ww.Status(),
				duration,
			)
		})
	}
}
