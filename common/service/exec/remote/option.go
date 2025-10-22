package remote

import (
	"context"

	"github.com/donknap/dpanel/common/service/ssh"
)

type Option func(command *Remote) error

func WithSSHClient(client *ssh.Client) Option {
	return func(self *Remote) error {
		self.client = client
		return nil
	}
}

func WithArgs(args ...string) Option {
	return func(self *Remote) error {
		self.Args = args
		return nil
	}
}

func WithCommandName(commandName string) Option {
	return func(self *Remote) error {
		if commandName == "" {
			return nil
		}
		self.Path = commandName
		return nil
	}
}

func WithDir(dir string) Option {
	return func(self *Remote) error {
		if dir == "" {
			return nil
		}
		self.Dir = dir
		return nil
	}
}

func WithEnv(env []string) Option {
	return func(self *Remote) error {
		self.Env = env
		return nil
	}
}

func WithCtx(ctx context.Context) Option {
	return func(self *Remote) error {
		self.ctx, self.ctxCancel = context.WithCancel(ctx)
		return nil
	}
}
