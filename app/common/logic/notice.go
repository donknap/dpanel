package logic

import (
	"crypto/tls"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/jordan-wright/email"
	"log/slog"
	"net/smtp"
	"strconv"
)

type Notice struct {
}

func (self Notice) Send(emailServer accessor.EmailServer, toEmail string, subject string, htmlContent string) error {
	e := email.NewEmail()
	e.From = emailServer.Email
	e.Subject = subject
	e.To = []string{toEmail}
	e.HTML = []byte(htmlContent)
	err := e.SendWithTLS(
		emailServer.Host+":"+strconv.Itoa(emailServer.Port),
		smtp.PlainAuth("", emailServer.Email, emailServer.Code, emailServer.Host),
		&tls.Config{
			ServerName: emailServer.Host, // 必须与证书匹配的域名
		},
	)
	if err != nil {
		slog.Debug("email send", "error", err)
	}
	return nil
}
