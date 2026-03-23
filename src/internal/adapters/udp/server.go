package udp

import (
	"context"
	"fmt"
	"net"
	"time"

	"xashloger/internal/adapters/udp/parser"
	"xashloger/internal/core/usecase"

	"github.com/sirupsen/logrus"
)

type Server struct {
	addr  string
	logic *usecase.HLDSService
}

func New(addr string, logic *usecase.HLDSService) *Server {
	return &Server{addr: addr, logic: logic}
}

func (s *Server) Run(ctx context.Context) error {
	udpAddr, err := net.ResolveUDPAddr("udp", s.addr)
	if err != nil {
		return fmt.Errorf("resolve address error: %w", err)
	}

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return fmt.Errorf("listen udp error: %w", err)
	}
	defer conn.Close()

	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()

	logrus.Infof("UDP HLDS server listening on %s", s.addr)

	buf := make([]byte, 2048)

	for {
		_ = conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		n, remoteAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			logrus.Warnf("read error: %v", err)
			continue
		}

		msg := string(buf[:n])

		logrus.Debugf("[RAW from %s] %q", remoteAddr, msg)

		entry, err := parser.ParseHLDSLog(msg)
		if err != nil {
			logrus.Warnf("parse error: %v | RAW: %q", err, msg)
			continue
		}

		entry.ServerIP = remoteAddr.String()

		if err := s.logic.Process(entry); err != nil {
			logrus.Errorf("service error: %v", err)
		}

		logrus.Infof(
			"[%s] [%s] [%s] [%s]",
			entry.Timestamp.Format("2006-01-02 15:04:05"),
			entry.Type,
			entry.Player,
			entry.Message,
		)
	}
}
