package service

import (
	"fmt"
	"strings"
	"time"
	"xashloger/internal/domain"
	"xashloger/internal/repository"
	"xashloger/internal/transport/udp/rcon"
	"xashloger/internal/utils/mailto"

	"github.com/sirupsen/logrus"
)

type HLDSService struct {
	repo        *repository.HLDSRepository
	mail        *mailto.MailTo
	ttlHours    int
	eventCount  int
	cleanupStep int
}

func NewHLDSService(repo *repository.HLDSRepository, mail *mailto.MailTo) *HLDSService {
	return &HLDSService{
		repo:        repo,
		mail:        mail,
		ttlHours:    48,
		cleanupStep: 10000,
	}
}

func (s *HLDSService) Process(entry *domain.LogEntry) error {

	autokick, err := s.repo.IsAutokickPlayer(entry.Player)
	if err != nil {
		logrus.Warnf("autokick check failed: %v", err)
	}

	if autokick {
		logrus.Infof("autokick player %s from %s", entry.Player, entry.ServerIP)

		go func() {
			rconClient := rcon.NewRCON(entry.ServerIP, "secret9")
			cmd := fmt.Sprintf("kick \"%s\"", strings.TrimSpace(entry.Player))
			if _, err := rconClient.Exec(cmd); err != nil {
				logrus.Warnf("RCON kick error: %v", err)
			}

			if entry.Player != "" {
				_ = s.repo.UpdateLastVisited(entry.Player, entry.Timestamp)
			}

			ev := &domain.Event{
				Timestamp: entry.Timestamp,
				Type:      entry.Type,
				Player:    entry.Player,
				Target:    entry.Target,
				Weapon:    entry.Weapon,
				Message:   entry.Message,
				Raw:       entry.Raw,
				SourceIP:  entry.SourceIP,
				ServerIP:  entry.ServerIP,
				TTL:       time.Now().Add(time.Duration(s.ttlHours) * time.Hour),
			}

			if err := s.repo.SaveEvent(ev); err != nil {
				logrus.Warnf("SaveEvent error: %v", err)
			}
		}()

		return nil
	}

	switch entry.Type {

	case "killed":
		s.repo.UpsertPlayer(entry.Player, 1, 0)
		s.repo.UpsertPlayer(entry.Target, 0, 1)

	case "suicide":
		s.repo.UpsertPlayer(entry.Player, -1, -1)

	case "connected":
		s.handleConnect(entry)

		// убрали фишку с трекером на disconnected
		// case "disconnected":
		// 	s.handleDisconnect(entry)
	}

	if entry.Player != "" {
		_ = s.repo.UpdateLastVisited(entry.Player, entry.Timestamp)
	}

	ev := &domain.Event{
		Timestamp: entry.Timestamp,
		Type:      entry.Type,
		Player:    entry.Player,
		Target:    entry.Target,
		Weapon:    entry.Weapon,
		Message:   entry.Message,
		Raw:       entry.Raw,
		SourceIP:  entry.SourceIP,
		ServerIP:  entry.ServerIP,
		TTL:       time.Now().Add(time.Duration(s.ttlHours) * time.Hour),
	}

	if err := s.repo.SaveEvent(ev); err != nil {
		return err
	}

	s.eventCount++
	if s.eventCount >= s.cleanupStep {
		s.eventCount = 0
		s.repo.CleanupEvents(s.ttlHours)
	}

	return nil
}

func (s *HLDSService) handleConnect(e *domain.LogEntry) {

	_, err := s.repo.GetUserForTracking(e.Player)
	if err != nil {
		return
	}

	subject := "Tracked player connected"
	body := fmt.Sprintf(
		"Tracked player connected\n\n"+
			"Name: %s\n"+
			"IP: %s\n"+
			"Server: %s\n"+
			"Time: %s\n",
		e.Player,
		e.SourceIP,
		e.ServerIP,
		e.Timestamp.Format("2006-01-02 15:04:05"),
	)

	go func() {
		if err := s.mail.Send(subject, body, "Tracked player connected"); err != nil {
			logrus.Warnf("mail error: %v", err)
		}
	}()
}

func (s *HLDSService) handleDisconnect(e *domain.LogEntry) {

	_, err := s.repo.GetUserForTracking(e.Player)
	if err != nil {
		return
	}

	subject := "Tracked player disconnected"
	body := fmt.Sprintf(
		"Tracked player disconnected\n\n"+
			"Name: %s\n"+
			"Server: %s\n"+
			"Time: %s\n",
		e.Player,
		e.ServerIP,
		e.Timestamp.Format("2006-01-02 15:04:05"),
	)

	go func() {
		if err := s.mail.Send(subject, body, "Tracked player disconnected"); err != nil {
			logrus.Warnf("mail error: %v", err)
		}
	}()
}
