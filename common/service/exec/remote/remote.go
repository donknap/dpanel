package remote

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/creack/pty"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/exec"
	"github.com/donknap/dpanel/common/service/ssh"
	ssh2 "golang.org/x/crypto/ssh"
	"io"
	"log/slog"
	"os"
	"strings"
)

func New(opts ...Option) (exec.Executor, error) {
	var err error
	c := &Remote{}

	for _, opt := range opts {
		err = opt(c)
		if err != nil {
			return nil, err
		}
	}

	if c.client == nil {
		return nil, errors.New("invalid ssh client")
	}

	if c.ctx == nil {
		c.ctx, c.ctxCancel = context.WithCancel(c.client.Ctx())
	}

	slog.Debug("run remote command", "ssh", c.client.Conn.RemoteAddr(), "name", c.String(), "env", c.Env)

	return c, nil
}

func QuickRun(client *ssh.Client, command string) ([]byte, error) {
	cmd, err := New(
		WithSSHClient(client),
		WithCommandName(command),
	)
	if err != nil {
		return nil, err
	}
	return cmd.RunWithResult()
}

type Remote struct {
	client    *ssh.Client
	Path      string
	Args      []string
	Env       []string
	Dir       string
	ctx       context.Context
	ctxCancel context.CancelFunc
}

func (self *Remote) AppendEnv(env []string) {
	self.Env = append(self.Env, env...)
}

func (self *Remote) AppendSystemEnv() {
	self.Env = append(self.Env, os.Environ()...)
}

func (self *Remote) Run() error {
	// session.Run 无法被上下文关闭，所以采用 Pip 代替
	session, err := self.client.NewSession()
	if err != nil {
		return err
	}
	if !function.IsEmptyArray(self.Env) {
		for _, s := range self.Env {
			if temp := strings.Split(s, "="); len(temp) > 1 {
				_ = session.Setenv(temp[0], strings.Join(temp[1:], "="))
			}
		}
	}
	pipeReader, pipeWriter := io.Pipe()

	go func() {
		select {
		case <-self.ctx.Done():
			if session != nil {
				_ = pipeWriter.CloseWithError(context.Canceled)
				_ = session.Close()
				_ = session.Signal(ssh2.SIGINT)
			}
		}
	}()

	session.Stderr = pipeWriter
	err = session.Start(self.String())
	if err != nil {
		return err
	}

	go func() {
		err := session.Wait()
		_ = pipeWriter.CloseWithError(err)
	}()

	result, err := io.ReadAll(pipeReader)
	if err != nil {
		return errors.Join(err, errors.New(string(result)))
	}

	if result != nil {
		return errors.New(string(result))
	}

	if self.ctx.Err() != nil {
		return context.Canceled
	}

	return nil
}

func (self *Remote) RunWithResult() ([]byte, error) {
	reader, err := self.RunInPip()
	if err != nil {
		return nil, err
	}
	result, err := io.ReadAll(reader)
	if err != nil {
		// 如果发生错误，并且 result 有值，result 为真正的错误信息， err 为执行状态，一般为  Process exited with status xxx
		if result != nil && len(result) > 0 {
			return nil, errors.New(string(result))
		}
		return nil, err
	}
	return bytes.TrimSpace(result), nil
}

func (self *Remote) RunInPip() (io.ReadCloser, error) {
	session, err := self.client.NewSession()
	if err != nil {
		return nil, err
	}
	if !function.IsEmptyArray(self.Env) {
		for _, s := range self.Env {
			if temp := strings.Split(s, "="); len(temp) > 1 {
				_ = session.Setenv(temp[0], strings.Join(temp[1:], "="))
			}
		}
	}
	pipeReader, pipeWriter := io.Pipe()

	go func() {
		select {
		case <-self.ctx.Done():
			if session != nil {
				_ = pipeWriter.CloseWithError(context.Canceled)
				_ = session.Close()
				_ = session.Signal(ssh2.SIGINT)
			}
		}
	}()

	r := &readCloser{
		buffer:  pipeReader,
		session: session,
	}

	session.Stdout = pipeWriter
	session.Stderr = pipeWriter

	err = session.Start(self.String())
	if err != nil {
		_ = pipeWriter.CloseWithError(err)
		_ = session.Close()
		return nil, err
	}

	go func() {
		err := session.Wait()
		_ = pipeWriter.CloseWithError(err)
	}()

	return r, nil
}

func (self *Remote) RunInTerminal(size *pty.Winsize) (io.ReadCloser, error) {
	session, reader, write, err := self.client.NewPtySession(24, 80)
	if err != nil {
		return nil, err
	}
	go func() {
		_, err = write.Write([]byte(self.String() + "\n"))
	}()
	return &readCloser{
		buffer:  reader,
		session: session,
	}, nil
}

func (self *Remote) Kill() error {
	self.ctxCancel()
	return nil
}

func (self *Remote) Close() error {
	self.ctxCancel()
	return nil
}

func (self *Remote) String() string {
	return fmt.Sprintf("%s %s", self.Path, strings.Join(self.Args, " "))
}
