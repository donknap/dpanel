package ssh

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"sync"

	"github.com/donknap/dpanel/common/function"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

const (
	SshAuthTypeBasic      = "basic"
	SshAuthTypePem        = "pem"
	SshAuthTypePemDefault = "pemDefault"
)

type Client struct {
	closeOnce       sync.Once
	Conn            *ssh.Client
	SftpConn        *sftp.Client
	ctx             context.Context
	ctxCancel       context.CancelFunc
	connectCtx      context.Context
	dial            func(context.Context, string, string) (net.Conn, error)
	needSftp        bool
	sshClientConfig *ssh.ClientConfig
	address         string
	protocol        string // 连接协议，tcp or tpc6
}
type Option func(self *Client) error

type ServerInfo struct {
	Username   string `json:"username,omitempty"`
	Password   string `json:"password,omitempty"`
	PrivateKey string `json:"privateKey,omitempty"`
	Address    string `json:"address,omitempty"`
	Port       int    `json:"port,omitempty"`
	AuthType   string `json:"authType,omitempty"`
}

var _ function.Sensitive[*ServerInfo] = (*ServerInfo)(nil)

func (self *ServerInfo) Encrypt(old *ServerInfo) error {
	if self == nil {
		return nil
	}
	info := *self
	if function.IsSensitivePlaceholder(info.Password) {
		if old == nil || old.Password == "" {
			return errors.New("SSH password is unavailable")
		}
		info.Password = old.Password
	}
	if function.IsSensitivePlaceholder(info.PrivateKey) {
		if old == nil || old.PrivateKey == "" {
			return errors.New("SSH private key is unavailable")
		}
		info.PrivateKey = old.PrivateKey
	}
	if info.Password != "" {
		if value, err := function.RSADecode(info.Password, nil); err == nil {
			info.Password = value
		}
		password, err := function.RSAEncode(info.Password)
		if err != nil {
			slog.Warn("SSH password encryption failed, keeping current value", "error", err)
		} else {
			info.Password = password
		}
	}
	if info.PrivateKey != "" {
		if value, err := function.RSADecode(info.PrivateKey, nil); err == nil {
			info.PrivateKey = value
		}
		privateKey, err := function.RSAEncode(info.PrivateKey)
		if err != nil {
			slog.Warn("SSH private key encryption failed, keeping current value", "error", err)
		} else {
			info.PrivateKey = privateKey
		}
	}
	*self = info
	return nil
}

func (self *ServerInfo) Decrypt(mask bool) (*ServerInfo, error) {
	if self == nil {
		return nil, nil
	}
	info := *self
	if value, err := function.RSADecode(info.Password, nil); err == nil {
		info.Password = value
	}
	if value, err := function.RSADecode(info.PrivateKey, nil); err == nil {
		info.PrivateKey = value
	}
	if mask {
		info.Password = function.MaskSensitiveValue(info.Password)
		info.PrivateKey = function.MaskSensitiveValue(info.PrivateKey)
	}
	return &info, nil
}
