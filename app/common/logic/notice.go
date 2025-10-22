package logic

import (
	"crypto/tls"
	"net/smtp"
	"strconv"

	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/function"
	"github.com/jordan-wright/email"
)

type Notice struct {
}

func (self Notice) Send(emailServer *accessor.NotificationEmailServer, toEmail string, subject string, htmlContent string) error {
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
		if function.ErrorHasKeyword(err, "535 Login fail") {
			return err
		}
		return nil
	}
	return nil
}
