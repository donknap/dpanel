package ssh

import (
	"context"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"io"
	"log/slog"
	"time"
)

func NewClient(opt ...Option) (*Client, error) {
	c := &Client{
		sshClientConfig: &ssh.ClientConfig{
			Timeout: time.Second * 5,
		},
	}
	var err error
	knownHostsCallback := NewDefaultKnownHostCallback()
	c.sshClientConfig.HostKeyCallback = knownHostsCallback.Handler

	for _, option := range opt {
		err := option(c)
		if err != nil {
			return nil, err
		}
	}

	if c.ctx == nil {
		c.ctx, c.ctxCancel = context.WithCancel(context.Background())
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

	go func() {
		<-c.ctx.Done()
		slog.Debug("ssh client close start")
		if c.SftpConn != nil {
			err = c.SftpConn.Close()
			if err != nil {
				slog.Debug("ssh sftp close", "error", err)
			}
		}
		if c.Conn != nil {
			err = c.Conn.Close()
			if err != nil {
				slog.Debug("ssh client close", "error", err)
			}
		}
	}()

	return c, nil
}

func (self *Client) NewSession() (*ssh.Session, error) {
	return self.Conn.NewSession()
}

func (self *Client) NewPtySession(height, width int) (session *ssh.Session, read io.Reader, write io.WriteCloser, err error) {
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
		return session, nil, nil, err
	}
	if err = session.RequestPty("xterm", height, width, ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}); err != nil {
		return session, nil, nil, err
	}
	write, err = session.StdinPipe()
	if err != nil {
		return session, nil, nil, err
	}
	read, err = session.StdoutPipe()
	if err != nil {
		return session, nil, nil, err
	}
	if stderr, err1 := session.StderrPipe(); err1 == nil {
		read = io.MultiReader(read, stderr)
	}
	if err = session.Shell(); err != nil {
		return session, nil, nil, err
	}
	return session, read, write, nil
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
	self.ctxCancel()
}

func (self *Client) Ctx() context.Context {
	return self.ctx
}
