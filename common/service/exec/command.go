package exec

import (
	"github.com/creack/pty"
	"io"
	"log/slog"
	"os"
	"os/exec"
)

var cmd *exec.Cmd

type Command struct {
}

type RunCommandOption struct {
	CmdName    string
	CmdArgs    []string
	WindowSize *pty.Winsize
}

func (self Command) RunInTerminal(option *RunCommandOption) {
	slog.Debug("run command", option.CmdName, option.CmdArgs)
	myWrite := &write{}
	cmd = exec.Command(option.CmdName, option.CmdArgs...)
	var out *os.File
	var err error
	if option.WindowSize != nil {
		out, err = pty.StartWithSize(cmd, option.WindowSize)
	} else {
		out, err = pty.Start(cmd)
	}
	if err != nil {
		slog.Debug(option.CmdName, err.Error())
	}
	_, _ = io.Copy(myWrite, out)
}

func (self Command) Run(option *RunCommandOption) {
	slog.Debug("run command", option.CmdName, option.CmdArgs)

	myWrite := &write{}
	cmd = exec.Command(option.CmdName, option.CmdArgs...)
	cmd.Stdout = myWrite
	cmd.Stderr = myWrite
	err := cmd.Run()
	if err != nil {
		slog.Debug(option.CmdName, err.Error())
	}
}

func (self Command) RunWithOut(option *RunCommandOption) string {
	slog.Debug("run command", option.CmdName, option.CmdArgs)
	cmd = exec.Command(option.CmdName, option.CmdArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		slog.Debug(option.CmdName, err.Error())
	}
	return string(out)
}

func (self Command) Kill() error {
	if cmd != nil {
		return cmd.Process.Kill()
	}
	return nil
}
