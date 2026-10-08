package local

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	exec2 "os/exec"
	"strings"

	"github.com/creack/pty"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/exec"
	"github.com/shirou/gopsutil/v4/process"
	"golang.org/x/text/encoding"
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
	cmd          *exec2.Cmd
	ctx          context.Context
	ctxCancel    context.CancelFunc
	terminalFile *os.File

	windowsEncoding encoding.Encoding
	quiet           bool
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

func (self *Local) RunInPip() (exec.Pipe, error) {
	self.debug()
	pr, pw := io.Pipe()
	stdinReader, stdinWriter := io.Pipe()
	stderrBuf := &bytes.Buffer{}
	self.cmd.Stdin = stdinReader
	self.cmd.Stdout = pw
	self.cmd.Stderr = io.MultiWriter(pw, stderrBuf)

	if err := self.cmd.Start(); err != nil {
		_ = pr.Close()
		_ = pw.Close()
		_ = stdinReader.Close()
		_ = stdinWriter.Close()
		return nil, err
	}

	go func() {
		err := self.cmd.Wait()
		if err != nil {
			err = fmt.Errorf("%s: %s", err.Error(), stderrBuf.String())
			slog.Debug("run command wait", "err", err)
			pw.CloseWithError(err)
		} else {
			pw.Close()
		}
		_ = stdinReader.Close()
	}()

	return readCloser{
		cmd:   self,
		Conn:  pr,
		stdin: stdinWriter,
	}, nil
}

func (self *Local) RunInReadPip() (io.ReadCloser, error) {
	self.debug()
	pr, pw := io.Pipe()
	stderrBuf := &bytes.Buffer{}
	self.cmd.Stdout = pw
	self.cmd.Stderr = io.MultiWriter(pw, stderrBuf)

	if err := self.cmd.Start(); err != nil {
		_ = pr.Close()
		_ = pw.Close()
		return nil, err
	}

	go func() {
		err := self.cmd.Wait()
		if err != nil {
			err = fmt.Errorf("%s: %s", err.Error(), stderrBuf.String())
			slog.Debug("run command wait", "err", err)
			_ = pw.CloseWithError(err)
		} else {
			_ = pw.Close()
		}
	}()

	return readCloser{
		cmd:  self,
		Conn: pr,
	}, nil
}

func (self *Local) ResizeTerminal(size *pty.Winsize) error {
	if self.terminalFile == nil || size == nil {
		return nil
	}
	return pty.Setsize(self.terminalFile, size)
}

func (self *Local) Kill() error {
	return self.Close()
}

func (self *Local) Close() error {
	if !self.quiet {
		slog.Debug("run command kill cmd", "cmd", self.cmd, "process", self.cmd.Process)
	}
	self.ctxCancel()
	return nil
}

func (self *Local) WorkDir(path string) {
	self.cmd.Dir = path
}

func (self *Local) debug() {
	if self.quiet {
		return
	}
	slog.Debug("run local command", "cmd", self.cmd.String(), "env", self.cmd.Env)
}
