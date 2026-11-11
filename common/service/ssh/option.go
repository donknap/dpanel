package ssh

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/donknap/dpanel/common/service/storage"
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

func WithAuthDefaultPem(username string, rsaKeyContent []byte) Option {
	return func(self *Client) error {
		return WithAuthPem(username, string(rsaKeyContent), "")(self)
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

// WithServerInfo 默认使用端口 22；未指定认证方式时，有私钥使用 pem，否则使用 basic。
func WithServerInfo(info *ServerInfo) Option {
	return func(self *Client) error {
		if info == nil || strings.TrimSpace(info.Address) == "" || strings.TrimSpace(info.Username) == "" || info.Port < 0 || info.Port > 65535 {
			return errors.New("invalid SSH server configuration")
		}
		// 配置可由多个连接共用，默认值和凭据解密只作用于本次副本。
		serverInfo, err := info.Decrypt(false)
		if err != nil {
			return err
		}
		if serverInfo.Port == 0 {
			serverInfo.Port = 22
		}
		if serverInfo.AuthType == "" {
			serverInfo.AuthType = SshAuthTypeBasic
			if serverInfo.PrivateKey != "" {
				serverInfo.AuthType = SshAuthTypePem
			}
		}
		if err := WithAddress(serverInfo.Address, serverInfo.Port)(self); err != nil {
			return err
		}
		switch serverInfo.AuthType {
		case SshAuthTypePem:
			return WithAuthPem(serverInfo.Username, serverInfo.PrivateKey, serverInfo.Password)(self)
		case SshAuthTypeBasic:
			return WithAuthBasic(serverInfo.Username, serverInfo.Password)(self)
		case SshAuthTypePemDefault:
			var rsaKeyContent []byte
			if v, ok := storage.Cache.Get(storage.CacheKeyRsaKey); ok {
				rsaKeyContent = v.([]byte)
			}
			return WithAuthDefaultPem(serverInfo.Username, rsaKeyContent)(self)
		default:
			return errors.New("unsupported SSH authentication type")
		}
	}
}

func WithSftpClient() Option {
	return func(self *Client) error {
		self.needSftp = true
		return nil
	}
}

// WithDial 指定底层拨号，nil 使用直连；返回的连接由本客户端关闭。
func WithDial(dial func(context.Context, string, string) (net.Conn, error)) Option {
	return func(self *Client) error {
		self.dial = dial
		return nil
	}
}

func WithContext(ctx context.Context) Option {
	return func(self *Client) error {
		self.ctx = ctx
		return nil
	}
}

// WithConnectContext 仅在 SSH 客户端建立完成前传递取消信号。
func WithConnectContext(ctx context.Context) Option {
	return func(self *Client) error {
		self.connectCtx = ctx
		return nil
	}
}

func WithTimeout(s time.Duration) Option {
	return func(self *Client) error {
		self.sshClientConfig.Timeout = s
		return nil
	}
}
