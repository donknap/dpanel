package exec

import (
	"context"
	"github.com/donknap/dpanel/common/function"
	"os/exec"
	"time"
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

func WithTimeout(timeout time.Duration) Option {
	return func(self *Command) error {
		// 配置超时间后
		ctx, _ := context.WithTimeout(context.Background(), timeout)
		self.cmd = exec.CommandContext(ctx, self.cmd.Args[0], self.cmd.Args[1:]...)
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
