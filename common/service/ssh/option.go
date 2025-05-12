package ssh

import (
	"fmt"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"strings"
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
		if password != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase([]byte(privateKeyPem), []byte(password))
		} else {
			signer, err = ssh.ParsePrivateKey([]byte(privateKeyPem))
		}
		if err != nil {
			return err
		}
		self.sshClientConfig.Auth = []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		}
		return nil
	}
}

func WithAddress(address string, port int) Option {
	return func(self *Client) error {
		self.address = fmt.Sprintf("%s:%d", address, port)
		if strings.Contains(address, ":") {
			self.protocol = "tcp6"
		} else {
			self.protocol = "tcp"
		}
		return nil
	}
}

func WithServerInfo(info *ServerInfo) []Option {
	option := make([]Option, 0)
	option = append(option, WithAddress(info.Host, info.Port))
	if info.AuthType == SshAuthTypePem {
		option = append(option, WithAuthPem(info.Username, info.PrivateKey, info.Password))
	} else if info.AuthType == SshAuthTypeBasic {
		option = append(option, WithAuthBasic(info.Username, info.Password))
	}
	return option
}

func WithSftpClient() Option {
	return func(self *Client) error {
		self.SftpConn = &sftp.Client{}
		return nil
	}
}
