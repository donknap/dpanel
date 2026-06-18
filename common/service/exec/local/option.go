package local

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/donknap/dpanel/common/function"
)

type Option func(command *Local) error

func WithArgs(args ...string) Option {
	return func(self *Local) error {
		var commandName string
		if self.cmd.Path == "" {
			commandName = args[0]
			args = args[1:]
		} else {
			commandName = self.cmd.Path
		}
		if strings.HasSuffix(commandName, "powershell.exe") && args[0] == "-c" {
			args[0] = "-Command"
		}
		self.cmd = exec.CommandContext(self.ctx, commandName, args...)
		return nil
	}
}

func WithCommandName(commandName string) Option {
	return func(self *Local) error {
		if commandName == "" {
			return nil
		}
		if (commandName == "/bin/sh" || commandName == "/bin/bash") && runtime.GOOS == "windows" {
			commandName = "powershell"
		}
		if function.IsEmptyArray(self.cmd.Args) {
			self.cmd = exec.CommandContext(self.ctx, commandName)
		} else {
			self.cmd = exec.CommandContext(self.ctx, commandName, self.cmd.Args...)
		}
		return nil
	}
}

func WithDefaultShell() Option {
	return func(self *Local) error {
		if runtime.GOOS == "windows" {
			for _, commandName := range []string{
				os.Getenv("ComSpec"),
				"powershell.exe",
				"powershell",
				"cmd.exe",
				"cmd",
			} {
				if commandName == "" {
					continue
				}
				if shellPath, err := exec.LookPath(commandName); err == nil {
					return WithCommandName(shellPath)(self)
				}
			}
			return WithCommandName("powershell")(self)
		}

		for _, commandName := range []string{
			os.Getenv("SHELL"),
			"/bin/sh",
			"/bin/bash",
			"sh",
			"bash",
		} {
			if commandName == "" {
				continue
			}
			if shellPath, err := exec.LookPath(commandName); err == nil {
				return WithCommandName(shellPath)(self)
			}
		}
		return WithCommandName("/bin/sh")(self)
	}
}

func WithDir(dir string) Option {
	return func(self *Local) error {
		if dir == "" {
			return nil
		}
		self.cmd.Dir = dir
		return nil
	}
}

func WithDefaultShellDir() Option {
	return func(self *Local) error {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil
		}
		return WithDir(homeDir)(self)
	}
}

func WithInteractiveTerminalEnv() Option {
	return func(self *Local) error {
		env := make([]string, 0, len(os.Environ())+2)
		for _, item := range os.Environ() {
			if strings.HasPrefix(item, "TERM=") || strings.HasPrefix(item, "COLORTERM=") {
				continue
			}
			env = append(env, item)
		}
		self.cmd.Env = append(env, "TERM=xterm-256color", "COLORTERM=truecolor")
		return nil
	}
}

func WithEnv(env []string) Option {
	return func(self *Local) error {
		self.cmd.Env = env
		return nil
	}
}

// WithCtx 保证最后调用
func WithCtx(ctx context.Context) Option {
	return func(self *Local) error {
		if function.IsEmptyArray(self.cmd.Args) {
			return errors.New("invalid arguments")
		}
		self.ctx, self.ctxCancel = context.WithCancel(ctx)
		newCmd := exec.CommandContext(self.ctx, self.cmd.Args[0], self.cmd.Args[1:]...)
		newCmd.Env = self.cmd.Env
		newCmd.Dir = self.cmd.Dir
		self.cmd = newCmd
		return nil
	}
}
