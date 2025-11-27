package local

import (
	"context"
	"errors"
	"os/exec"

	"github.com/donknap/dpanel/common/function"
)

type Option func(command *Local) error

func WithArgs(args ...string) Option {
	return func(self *Local) error {
		if self.cmd.Path == "" {
			self.cmd = exec.CommandContext(self.ctx, args[0], args[1:]...)
		} else {
			self.cmd = exec.CommandContext(self.ctx, self.cmd.Path, args...)
		}
		return nil
	}
}

func WithCommandName(commandName string) Option {
	return func(self *Local) error {
		if commandName == "" {
			return nil
		}
		if function.IsEmptyArray(self.cmd.Args) {
			self.cmd = exec.CommandContext(self.ctx, commandName)
		} else {
			self.cmd = exec.CommandContext(self.ctx, commandName, self.cmd.Args...)
		}
		return nil
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
