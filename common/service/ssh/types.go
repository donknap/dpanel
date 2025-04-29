package ssh

import (
	"context"
	"golang.org/x/crypto/ssh"
)

const (
	SshAuthTypeBasic = "basic"
	SshAuthTypePem   = "pem"
)

type Client struct {
	Conn            *ssh.Client
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
	Host       string `json:"host,omitempty"`
	Port       int    `json:"port,omitempty"`
	AuthType   string `json:"authType,omitempty"`
}
