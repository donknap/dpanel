package exec

import (
	"context"
	"errors"
	"github.com/donknap/dpanel/common/function"
	"os/exec"
)

type Option func(command *Command) error

func WithArgs(args ...string) Option {
	return func(self *Command) error {
		if self.cmd.Path == "" {
			self.cmd = exec.Command(args[0], args[1:]...)
		} else {
			self.cmd = exec.Command(self.cmd.Path, args...)
		}
		return nil
	}
}

func WithCommandName(commandName string) Option {
	return func(self *Command) error {
		if commandName == "" {
			return nil
		}
		if function.IsEmptyArray(self.cmd.Args) {
			self.cmd = exec.Command(commandName)
		} else {
			self.cmd = exec.Command(commandName, self.cmd.Args...)
		}
		return nil
	}
}

func WithDir(dir string) Option {
	return func(self *Command) error {
		if dir == "" {
			return nil
		}
		self.cmd.Dir = dir
		return nil
	}
}

func WithEnv(env []string) Option {
	return func(self *Command) error {
		self.cmd.Env = env
		return nil
	}
}

func WithCtx(ctx context.Context) Option {
	return func(self *Command) error {
		if function.IsEmptyArray(self.cmd.Args) {
			return errors.New("invalid arguments")
		}
		newCmd := exec.CommandContext(ctx, self.cmd.Args[0], self.cmd.Args[1:]...)
		newCmd.Env = self.cmd.Env
		newCmd.Dir = self.cmd.Dir
		self.cmd = newCmd
		return nil
	}
}
