package local

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	exec2 "os/exec"
	"runtime"
	"strings"

	"github.com/creack/pty"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/exec"
	"github.com/shirou/gopsutil/v4/process"
)

func New(opts ...Option) (exec.Executor, error) {
	var err error

	ctx, cancel := context.WithCancel(context.Background())

	c := &Local{
		cmd: &exec2.Cmd{
			Env: make([]string, 0),
		},
		ctx:       ctx,
		ctxCancel: cancel,
	}

	for _, opt := range opts {
		err = opt(c)
		if err != nil {
			return nil, err
		}
	}

	return c, nil
}

func QuickRun(cmdStrOrArr ...string) ([]byte, error) {
	if function.IsEmptyArray(cmdStrOrArr) {
		return nil, errors.New("invalid cmd")
	}
	if _, ok := function.IndexArrayWalk(cmdStrOrArr, func(item string) bool {
		return strings.Contains(item, " ")
	}); ok {
		cmdStrOrArr = function.SplitCommandArray(strings.Join(cmdStrOrArr, " "))
	}
	cmd, err := New(
		WithCommandName(cmdStrOrArr[0]),
		WithArgs(cmdStrOrArr[1:]...),
	)
	if err != nil {
		return nil, err
	}
	return cmd.RunWithResult()
}

func QuickCheckRunning(targetCmd string) (bool, error) {
	currentPID := int32(os.Getpid())

	processes, err := process.Processes()
	if err != nil {
		return false, err
	}

	for _, p := range processes {
		if p.Pid == currentPID {
			continue
		}
		name, err := p.Name()
		if err == nil {
			if strings.EqualFold(strings.ToLower(name), strings.ToLower(targetCmd)) {
				return true, nil
			}
		}
	}

	return false, nil
}

type Local struct {
	cmd       *exec2.Cmd
	ctx       context.Context
	ctxCancel context.CancelFunc
}

func (self *Local) AppendEnv(env []string) {
	self.cmd.Env = append(self.cmd.Env, env...)
}

func (self *Local) AppendSystemEnv() {
	self.cmd.Env = append(self.cmd.Env, os.Environ()...)
}

func (self *Local) String() string {
	return self.cmd.String()
}

func (self *Local) Run() error {
	self.debug()
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
	self.debug()
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
	self.debug()
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
			slog.Debug("run command wait", "err", err)
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

	self.debug()
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
	slog.Debug("run command kill cmd", "cmd", self.cmd, "process", self.cmd.Process)
	self.ctxCancel()
	return nil
}

func (self *Local) WorkDir(path string) {
	self.cmd.Dir = path
}

func (self *Local) debug() {
	slog.Debug("run local command", "cmd", self.cmd.String(), "env", self.cmd.Env)
}
