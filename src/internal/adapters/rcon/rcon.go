package rcon

import (
	"bytes"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

type RCONClient struct {
	addr     string
	password string
	timeout  time.Duration
}

func NewRCON(addr, password string) *RCONClient {
	logrus.WithFields(logrus.Fields{
		"addr": addr,
	}).Info("Creating new RCON client")
	return &RCONClient{
		addr:     addr,
		password: password,
		timeout:  1 * time.Second,
	}
}

func (c *RCONClient) Exec(cmd string) (string, error) {
	conn, err := net.DialTimeout("udp", c.addr, c.timeout)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	logrus.Infof("[RCON] Executing command: %s", cmd)

	payload := fmt.Sprintf("\xff\xff\xff\xffrcon %s %s\n", c.password, cmd)
	if _, err := conn.Write([]byte(payload)); err != nil {
		return "", err
	}
	logrus.Debugf("[RCON] Sent payload: %v", []byte(payload))

	var out bytes.Buffer
	_ = conn.SetReadDeadline(time.Now().Add(c.timeout))
	buf := make([]byte, 512)

	for {
		n, err := conn.Read(buf)
		if err != nil {
			logrus.Debugf("[RCON] Read error or timeout: %v", err)
			break
		}

		if n <= 4 {
			continue
		}

		chunk := strings.TrimSpace(string(buf[4:n]))
		if chunk == "print" || chunk == "" {
			continue
		}

		logrus.Debugf("[RCON] Received chunk: %q", chunk)
		out.WriteString(chunk)
		out.WriteByte('\n')
	}

	resp := strings.TrimSpace(out.String())
	logrus.Infof("[RCON] Command response: %q", resp)
	return resp, nil
}
