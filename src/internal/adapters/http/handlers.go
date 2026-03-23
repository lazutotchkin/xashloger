package http

import (
	"net"
	"strconv"

	"xashloger/internal/adapters/http/controllers"
	"xashloger/internal/adapters/http/models"
	"xashloger/internal/adapters/http/web"
	"xashloger/internal/core/ports"
	"xashloger/internal/infra/config"

	"gorm.io/gorm"
)

type HTTPServer struct {
	cfg       *config.Config
	db        *gorm.DB
	adminRepo ports.AdminRepository
	renderer  web.Renderer

	playersCtrl *controllers.PlayersController
	eventsCtrl  *controllers.EventsController
	adminCtrl   *controllers.AdminController
}

func NewHTTPServer(db *gorm.DB, adminRepo ports.AdminRepository, cfg *config.Config) *HTTPServer {
	serverInfos := make(map[string]models.ServerInfo)
	for _, srv := range cfg.TrackingServers {
		if !srv.Enabled {
			continue
		}

		addr := net.JoinHostPort(srv.Addr, strconv.Itoa(srv.Port))
		serverInfos[addr] = models.ServerInfo{
			Name: srv.Name,
			RCON: srv.RCONPassword,
		}
	}

	server := &HTTPServer{
		db:        db,
		adminRepo: adminRepo,
		renderer:  web.NewRenderer(),
		cfg:       cfg,
	}

	server.playersCtrl = controllers.NewPlayersController(models.NewPlayersModel(db), server.renderer)
	server.eventsCtrl = controllers.NewEventsController(models.NewEventsModel(db), server.renderer)
	server.adminCtrl = controllers.NewAdminController(models.NewAdminModel(adminRepo, serverInfos), server.renderer)

	return server
}
