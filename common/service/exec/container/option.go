package container

import (
	"context"

	containertypes "github.com/docker/docker/api/types/container"
	dockerclient "github.com/docker/docker/client"
)

type Option func(*Container) error

func WithDockerClient(client *dockerclient.Client) Option {
	return func(self *Container) error { self.client = client; return nil }
}

func WithContainerName(name string) Option {
	return func(self *Container) error { self.containerName = name; return nil }
}

func WithCommandName(name string) Option {
	return func(self *Container) error { self.command = append([]string{name}, self.command...); return nil }
}

func WithArgs(args ...string) Option {
	return func(self *Container) error { self.command = append(self.command, args...); return nil }
}

func WithEnv(env []string) Option {
	return func(self *Container) error { self.env = append(self.env, env...); return nil }
}

func WithCtx(ctx context.Context) Option {
	return func(self *Container) error {
		self.cancel()
		self.ctx, self.cancel = context.WithCancel(ctx)
		return nil
	}
}

func WithExecOptions(option containertypes.ExecOptions) Option {
	return func(self *Container) error {
		self.option = option
		return nil
	}
}
