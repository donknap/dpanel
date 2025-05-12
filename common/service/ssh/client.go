package ssh

import (
	"context"
	"fmt"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"io"
	"strings"
	"time"
)

func NewClient(opt ...Option) (*Client, error) {
	c := &Client{
		sshClientConfig: &ssh.ClientConfig{
			Timeout: time.Second * 5,
		},
	}
	c.ctx, c.ctxCancel = context.WithCancel(context.Background())
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
	c.Conn, err = ssh.Dial(c.protocol, c.address, c.sshClientConfig)
	if err != nil {
		return nil, err
	}

	if c.SftpConn != nil {
		c.SftpConn, err = c.NewSftpSession()
		if err != nil {
			c.Close()
			return nil, err
		}
	}

	return c, nil
}

func (self *Client) RunContext(ctx context.Context, name string, args ...string) (string, error) {
	session, err := self.Conn.NewSession()
	if err != nil {
		return "", err
	}
	sessionCtx, sessionCtxCancel := context.WithCancel(ctx)
	defer func() {
		sessionCtxCancel()
	}()

	cmd := fmt.Sprintf("%s %s", name, strings.Join(args, " "))
	result, err := session.CombinedOutput(cmd)
	if err != nil {
		return "", err
	}
	go func() {
		select {
		case <-sessionCtx.Done():
			if session != nil {
				_ = session.Close()
				_ = session.Signal(ssh.SIGINT)
			}
		}
	}()
	return string(result), nil
}

func (self *Client) Run(name string, args ...string) (string, error) {
	return self.RunContext(context.Background(), name, args...)
}

func (self *Client) NewSession() (*ssh.Session, error) {
	return self.Conn.NewSession()
}

func (self *Client) NewPtySession(height, width int) (read io.Reader, write io.WriteCloser, err error) {
	var session *ssh.Session

	go func() {
		select {
		case <-self.ctx.Done():
			if write != nil {
				_ = write.Close()
			}
			if session != nil {
				_ = session.Close()
				_ = session.Signal(ssh.SIGINT)
			}
		}
	}()

	session, err = self.NewSession()
	if err != nil {
		return nil, nil, err
	}

	if err = session.RequestPty("xterm", height, width, ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}); err != nil {
		return nil, nil, err
	}
	write, err = session.StdinPipe()
	if err != nil {
		return nil, nil, err
	}
	read, err = session.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	if stderr, err1 := session.StderrPipe(); err1 == nil {
		read = io.MultiReader(read, stderr)
	}
	if err = session.Shell(); err != nil {
		return nil, nil, err
	}
	return read, write, nil
}

func (self *Client) NewSftpSession() (*sftp.Client, error) {
	sftpClient, err := sftp.NewClient(self.Conn)
	if err != nil {
		return nil, err
	}
	go func() {
		select {
		case <-self.ctx.Done():
			_ = sftpClient.Close()
		}
	}()
	return sftpClient, nil
}

func (self *Client) Close() {
	if self.Conn != nil {
		_ = self.Conn.Close()
	}
	self.ctxCancel()
}
