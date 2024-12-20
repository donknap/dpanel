package exec

import (
	"bytes"
	"context"
	"errors"
	"github.com/creack/pty"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"time"
)

var cmd *exec.Cmd

type Command struct {
}

type RunCommandOption struct {
	CmdName    string
	CmdArgs    []string
	WindowSize *pty.Winsize
	Timeout    time.Duration
	Dir        string
	Env        []string
}

func (self Command) RunInTerminal(option *RunCommandOption) (io.ReadCloser, error) {
	cmd = self.getCommand(option)
	var out *os.File
	var err error

	if option.WindowSize != nil {
		out, err = pty.StartWithSize(cmd, option.WindowSize)
	} else {
		out, err = pty.Start(cmd)
	}

	if err != nil {
		return nil, err
	}
	return TerminalResult{
		Conn: out,
		cmd:  cmd,
	}, err
}

func (self Command) Run(option *RunCommandOption) (io.Reader, error) {
	out := new(bytes.Buffer)
	cmd = self.getCommand(option)
	cmd.Stdout = out
	cmd.Stderr = out
	err := cmd.Run()
	if err != nil {
		return nil, errors.Join(err, errors.New(out.String()))
	}
	return out, nil
}

func (self Command) RunWithResult(option *RunCommandOption) string {
	cmd = self.getCommand(option)
	out, err := cmd.CombinedOutput()
	if err != nil {
		slog.Debug(option.CmdName, "arg", option.CmdArgs, "error", err.Error())
	}
	return string(out)
}

func (self Command) getCommand(option *RunCommandOption) *exec.Cmd {
	// 配置了超时退出，则不杀掉上一个进程
	if option.Timeout == 0 && cmd != nil && cmd.Process != nil && cmd.Process.Pid > 0 {
		slog.Debug("command kill debug", "cmd", cmd, "process", cmd.Process, "pid", cmd.Process.Pid)
		// 将上一条命令中止掉
		if err := cmd.Process.Kill(); err == nil {
			_, err = cmd.Process.Wait()
			if err != nil {
				slog.Debug("command kill", "error", err.Error())
			}
		}
	}
	slog.Debug("run command", option.CmdName, option.CmdArgs)
	cmd = exec.Command(option.CmdName, option.CmdArgs...)
	if option.Timeout > 0 {
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(option.Timeout))
		go func() {
			select {
			case <-ctx.Done():
				cancel()
				err := cmd.Process.Kill()
				if err != nil {
					slog.Error("run command timeout error", err)
				}
			}
		}()
	}
	if option.Dir != "" {
		cmd.Dir = option.Dir
	}
	if option.Env != nil {
		cmd.Env = option.Env
	}
	return cmd
}

func (self Command) Kill() error {
	if cmd != nil {
		err := cmd.Process.Kill()
		if err != nil {
			return err
		}
		return cmd.Wait()
	}
	return nil
}
