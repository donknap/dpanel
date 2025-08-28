package local

import (
	"bytes"
	"context"
	"errors"
	"github.com/creack/pty"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/exec"
	"io"
	"log/slog"
	"os"
	exec2 "os/exec"
	"runtime"
)

func New(opts ...Option) (exec.Executor, error) {
	var err error

	ctx, cancel := context.WithCancel(context.Background())

	c := &Local{
		cmd:       &exec2.Cmd{},
		ctx:       ctx,
		ctxCancel: cancel,
	}

	for _, opt := range opts {
		err = opt(c)
		if err != nil {
			return nil, err
		}
	}

	slog.Debug("run local command", "name", c.cmd.Path, "args", c.cmd.Args[1:], "env", c.cmd.Env)

	return c, nil
}

func QuickRun(command string) ([]byte, error) {
	cmdArr := function.SplitCommandArray(command)
	cmd, err := New(
		WithCommandName(cmdArr[0]),
		WithArgs(cmdArr[1:]...),
	)
	if err != nil {
		return nil, err
	}
	return cmd.RunWithResult()
}

type Local struct {
	cmd       *exec2.Cmd
	ctx       context.Context
	ctxCancel context.CancelFunc
}

func (self *Local) String() string {
	return self.cmd.String()
}

func (self *Local) Run() error {
	out := new(bytes.Buffer)
	self.cmd.Stderr = out
	err := self.cmd.Run()
	if err != nil {
		return errors.Join(errors.New(out.String()), err)
	}
	if out.Len() > 0 {
		return errors.New(out.String())
	}
	return nil
}

func (self *Local) RunWithResult() ([]byte, error) {
	out, err := self.cmd.CombinedOutput()
	if err != nil {
		if out != nil && len(out) > 0 {
			return nil, errors.New(string(out))
		}
		return nil, err
	}
	return out, nil
}

func (self *Local) RunInPip() (io.ReadCloser, error) {
	stdout, err := self.cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	self.cmd.Stderr = self.cmd.Stdout
	if err = self.cmd.Start(); err != nil {
		return nil, err
	}
	go func() {
		err = self.cmd.Wait()
		if err != nil {
			slog.Debug("run command wait", "err", "error")
		}
	}()
	return readCloser{
		cmd:  self,
		Conn: stdout,
	}, nil
}

func (self *Local) RunInTerminal(size *pty.Winsize) (io.ReadCloser, error) {
	var out *os.File
	var err error

	if runtime.GOOS == "windows" {
		// 不支持 Pty，利用管道模拟读取
		return self.RunInPip()
	}

	out, err = pty.StartWithSize(self.cmd, size)
	if err != nil {
		return nil, err
	}
	return TerminalResult{
		Conn: out,
		cmd:  self.cmd,
	}, err
}

func (self *Local) Kill() error {
	return self.Close()
}

func (self *Local) Close() error {
	slog.Debug("run command kill cmd", "cmd", self.cmd)
	self.ctxCancel()
	return nil
}
