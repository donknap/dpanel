package ssh

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/donknap/dpanel/common/service/storage"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

func WithAuthBasic(username, password string) Option {
	return func(self *Client) error {
		self.sshClientConfig.User = username
		self.sshClientConfig.Auth = []ssh.AuthMethod{
			ssh.Password(password),
		}
		return nil
	}
}

func WithAuthPem(username string, privateKeyPem string, password string) Option {
	return func(self *Client) error {
		var signer ssh.Signer
		var err error
		self.sshClientConfig.User = username
		signer, err = ssh.ParsePrivateKey([]byte(privateKeyPem))
		if err != nil {
			if password == "" {
				return err
			}
			signer, err = ssh.ParsePrivateKeyWithPassphrase([]byte(privateKeyPem), []byte(password))
			if err != nil {
				return err
			}
		}
		self.sshClientConfig.Auth = []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		}
		return nil
	}
}

func WithAuthDefaultPem(username string) Option {
	return func(self *Client) error {
		_, private, err := storage.GetCertRsaContent()
		if err != nil {
			return err
		}
		return WithAuthPem(username, string(private), "")(self)
	}
}

func WithAddress(address string, port int) Option {
	return func(self *Client) error {
		if strings.Contains(address, ":") {
			address = strings.Trim(strings.Trim(address, "["), "]")
			self.address = fmt.Sprintf("[%s]:%d", address, port)
			self.protocol = "tcp6"
		} else {
			self.address = fmt.Sprintf("%s:%d", address, port)
			self.protocol = "tcp"
		}
		return nil
	}
}

func WithServerInfo(info *ServerInfo) []Option {
	option := make([]Option, 0)
	option = append(option, WithAddress(info.Address, info.Port))
	if info.AuthType == SshAuthTypePem {
		option = append(option, WithAuthPem(info.Username, info.PrivateKey, info.Password))
	} else if info.AuthType == SshAuthTypeBasic {
		option = append(option, WithAuthBasic(info.Username, info.Password))
	} else if info.AuthType == SshAuthTypePemDefault {
		option = append(option, WithAuthDefaultPem(info.Username))
	}
	return option
}

func WithSftpClient() Option {
	return func(self *Client) error {
		self.SftpConn = &sftp.Client{}
		return nil
	}
}

func WithContext(ctx context.Context) Option {
	return func(self *Client) error {
		self.ctx, self.ctxCancel = context.WithCancel(ctx)
		return nil
	}
}

func WithTimeout(s time.Duration) Option {
	return func(self *Client) error {
		self.sshClientConfig.Timeout = s
		return nil
	}
}
