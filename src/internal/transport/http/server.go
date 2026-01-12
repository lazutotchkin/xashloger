package http

import (
	"net/http"
	"time"
	"xashloger/internal/transport/http/custom_middleware"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
	"github.com/sirupsen/logrus"
)

func (s *HTTPServer) Run(addr string) error {
	go StartCPUCollector()

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(s.logrusMiddleware(logrus.StandardLogger()))
	r.Use(middleware.Recoverer)
	r.Use(httprate.LimitByIP(5, 1*time.Second))

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
			Get("/events", s.EventsPage)
		r.With(ipFilter.Middleware).
			Get("/admin", s.AdminPage)
		r.With(ipFilter.Middleware).
			Post("/admin/kick", s.KickPlayer)
		r.With(ipFilter.Middleware).
			Post("/admin/untrack", s.UntrackPlayer)
		r.With(ipFilter.Middleware).
			Post("/admin/track", s.TrackPlayer)
		r.With(ipFilter.Middleware).
			Post("/admin/kick", s.KickPlayer)

		r.With(ipFilter.Middleware).
			Post("/admin/autokick/add", s.AutoKickPlayerAdd)
		r.With(ipFilter.Middleware).
			Post("/admin/autokick/remove", s.AutoKickPlayerRemove)

		r.With(ipFilter.Middleware).
			Get("/api/admin", s.AdminAPI)
	} else {
		r.Get("/events", s.EventsPage)
	}

	r.Get("/players", s.PlayersPage)

	return http.ListenAndServe(addr, r)
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
