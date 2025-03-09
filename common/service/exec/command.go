package exec

import (
	"bytes"
	"errors"
	"github.com/creack/pty"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"runtime"
)

var cmd *exec.Cmd

func New(opts ...Option) (*Command, error) {
	var err error
	c := &Command{
		cmd: &exec.Cmd{},
	}

	for _, opt := range opts {
		err = opt(c)
		if err != nil {
			return nil, err
		}
	}

	slog.Debug("run command", "args", c.cmd.Args, "env", c.cmd.Env)

	if c.cmd.Cancel == nil {
		// 没有配置超时时间，则先杀掉上一个进程
		// 如果配置了超时时间，自行处理命令的终止问题，不会在下次执行命令时被清理掉
		_ = Kill()
		cmd = c.cmd
	}

	return c, nil
}

type Command struct {
	cmd *exec.Cmd
}

func (self Command) RunInTerminal(size *pty.Winsize) (io.ReadCloser, error) {
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

func (self Command) RunInPip() (stdout io.ReadCloser, err error) {
	stdout, err = self.cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	self.cmd.Stderr = self.cmd.Stdout
	if err = self.cmd.Start(); err != nil {
		return nil, err
	}
	return stdout, nil
}

func (self Command) Run() (io.Reader, error) {
	out := new(bytes.Buffer)
	self.cmd.Stdout = out
	self.cmd.Stderr = out
	err := self.cmd.Run()
	if err != nil {
		return nil, errors.Join(err, errors.New(out.String()))
	}
	return out, nil
}

func (self Command) RunWithResult() string {
	out, err := self.cmd.CombinedOutput()
	if err != nil {
		slog.Debug("run command with result", "arg", self.cmd.Args, "error", err.Error())
		return string(out)
	}
	return string(out)
}

func Kill() error {
	var err error

	if cmd != nil && cmd.Process != nil && cmd.Process.Pid > 0 {
		err = cmd.Process.Kill()
		if err == nil {
			_, _ = cmd.Process.Wait()
		}
		slog.Debug("run command kill global cmd", "pid", cmd.Process.Pid, "name", cmd.String(), "error", err)
	}
	return nil
}

func (self Command) Cmd() *exec.Cmd {
	return self.cmd
}
