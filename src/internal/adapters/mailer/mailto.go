package mailto

import (
	"fmt"
	"net/smtp"
	"strings"
	"xashloger/internal/infra/config"

	"github.com/sirupsen/logrus"
)

type MailTo struct {
	SMTPHost string
	SMTPPort string
	From     string
	To       []string
	ToError  []string
}

func New(cfg *config.Config) *MailTo {
	return &MailTo{
		SMTPHost: cfg.Mail.SMTPHost,
		SMTPPort: cfg.Mail.SMTPPort,
		From:     cfg.Mail.From,
		To:       cfg.Mail.To,
		ToError:  cfg.Mail.ToError,
	}
}

func (m *MailTo) Send(subject, body string, logMessage string) error {
	return m.sendTo(m.To, subject, body, logMessage)
}

func (m *MailTo) SendError(subject, body string) error {
	recipients := m.ToError
	if len(recipients) == 0 {
		recipients = m.To
	}
	return m.sendTo(recipients, subject, body, "Send error email")
}

func (m *MailTo) sendTo(recipients []string, subject, body, logMessage string) error {
	addr := fmt.Sprintf("%s:%s", m.SMTPHost, m.SMTPPort)

	if len(recipients) == 0 {
		logrus.Error("No recipients specified in config")
		return fmt.Errorf("no recipients specified")
	}

	toHeader := strings.Join(recipients, ", ")
	msg := []byte(
		"To: " + strings.TrimSpace(toHeader) + "\r\n" +
			"From: " + strings.TrimSpace(m.From) + "\r\n" +
			"Subject: " + strings.TrimSpace(subject) + "\r\n\r\n" +
			body,
	)

	if logMessage == "" {
		logMessage = fmt.Sprintf("Send email to %v with subject '%s'", recipients, subject)
	}

	logrus.Info(logMessage)

	err := smtp.SendMail(addr, nil, m.From, recipients, msg)
	if err != nil {
		logrus.Warnf("SMTP send error: %v", err)
		return err
	}

	return nil
}
