package ssh

import (
	"context"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

const (
	SshAuthTypeBasic      = "basic"
	SshAuthTypePem        = "pem"
	SshAuthTypePemDefault = "pemDefault"
)

type Client struct {
	Conn            *ssh.Client
	SftpConn        *sftp.Client
	ctx             context.Context
	ctxCancel       context.CancelFunc
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
