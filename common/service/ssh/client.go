package ssh

import (
	"context"
	"github.com/donknap/dpanel/common/service/storage"
	"golang.org/x/crypto/ssh"
	"time"
)

func NewClient(opt ...Option) (*Client, error) {
	c := &Client{
		sshClientConfig: &ssh.ClientConfig{
			Timeout: time.Second * 10,
		},
	}
	var err error
	knownHostsCallback := DefaultKnownHostsCallback{
		path: storage.Local{}.GetSshKnownHostsPath(),
	}
	c.sshClientConfig.HostKeyCallback = knownHostsCallback.Handler

	for _, option := range opt {
		err := option(c)
		if err != nil {
			return nil, err
		}
	}
	c.sshClient, err = ssh.Dial(c.protocol, c.address, c.sshClientConfig)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (self *Client) RunContext(ctx context.Context, name string, args ...string) (string, error) {
	//session, err := self.sshClient.NewSession()
	//if err != nil {
	//	return "", err
	//}
	return "", nil
}

func (self *Client) Run(name string, args ...string) {

}
