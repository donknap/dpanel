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
	Follow     bool
	Timeout    time.Duration
}

func (self Command) RunInTerminal(option *RunCommandOption) (io.ReadCloser, error) {
	slog.Debug("run command", option.CmdName, option.CmdArgs)

	cmd = exec.Command(option.CmdName, option.CmdArgs...)
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
	slog.Debug("run command", option.CmdName, option.CmdArgs)
	out := new(bytes.Buffer)
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
	cmd.Stdout = out
	cmd.Stderr = out
	err := cmd.Run()
	if err != nil {
		return nil, errors.Join(err, errors.New(out.String()))
	}
	return out, nil
}

func (self Command) RunWithResult(option *RunCommandOption) string {
	slog.Debug("run command", option.CmdName, option.CmdArgs)
	cmd = exec.Command(option.CmdName, option.CmdArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		slog.Debug(option.CmdName, option.CmdArgs, err.Error())
	}
	return string(out)
}

func (self Command) Kill() error {
	if cmd != nil {
		return cmd.Process.Kill()
	}
	return nil
}
