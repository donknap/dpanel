package ssh

import "golang.org/x/crypto/ssh"

type Client struct {
	sshClientConfig *ssh.ClientConfig
	sshClient       *ssh.Client
	address         string
	protocol        string // 连接协议，tcp or tpc6
}
type Option func(self *Client) error
