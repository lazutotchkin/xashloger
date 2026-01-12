package mailto

import (
	"fmt"
	"net/smtp"
	"strings"
	"xashloger/internal/config"

	"github.com/sirupsen/logrus"
)

type MailTo struct {
	SMTPHost string
	SMTPPort string
	From     string
	To       []string
}

func New(cfg *config.Config) *MailTo {
	return &MailTo{
		SMTPHost: cfg.Mail.SMTPHost,
		SMTPPort: cfg.Mail.SMTPPort,
		From:     cfg.Mail.From,
		To:       cfg.Mail.To,
	}
}

func (m *MailTo) Send(subject, body string, logMessage string) error {
	addr := fmt.Sprintf("%s:%s", m.SMTPHost, m.SMTPPort)

	if len(m.To) == 0 {
		logrus.Error("No recipients specified in config")
		return fmt.Errorf("no recipients specified")
	}

	toHeader := strings.Join(m.To, ", ")
	msg := []byte(
		"To: " + strings.TrimSpace(toHeader) + "\r\n" +
			"From: " + strings.TrimSpace(m.From) + "\r\n" +
			"Subject: " + strings.TrimSpace(subject) + "\r\n\r\n" +
			body,
	)

	if logMessage == "" {
		logMessage = fmt.Sprintf("Send email to %v with subject '%s'", m.To, subject)
	}

	logrus.Info(logMessage)

	err := smtp.SendMail(addr, nil, m.From, m.To, msg)
	if err != nil {
		logrus.Warnf("SMTP send error: %v", err)
		return err
	}

	return nil
}
