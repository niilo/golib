package smtp

import (
	"bytes"
	"fmt"
	"html/template"
	"net/smtp"
	"strings"
)

type SmtpServer struct {
	Host     string
	Port     int
	Username string
	Passwd   string
}

func (s *SmtpServer) SendEmail(email Email) error {

	parameters := &struct {
		From    string
		To      string
		Subject string
		Message string
	}{
		email.From,
		strings.Join([]string(email.To), ","),
		email.Title,
		email.Message,
	}

	buffer := new(bytes.Buffer)

	template := template.Must(template.New("emailTemplate").Parse(emailTemplate))
	template.Execute(buffer, parameters)

	auth := smtp.PlainAuth("", s.Username, s.Passwd, s.Host)

	err := smtp.SendMail(
		fmt.Sprintf("%s:%d", s.Host, s.Port),
		auth,
		email.From,
		email.To,
		buffer.Bytes())

	return err
}

type Email struct {
	From    string
	To      []string
	Title   string
	Message string
}

const emailTemplate = `From: {{.From}}
To: {{.To}}
Subject: {{.Subject}}
MIME-version: 1.0
Content-Type: text/html; charset="UTF-8"

{{.Message}}`
