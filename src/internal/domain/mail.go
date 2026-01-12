package domain

type MailTo struct {
	SMTPHost string
	SMTPPort string
	From     string
	To       []string
}
