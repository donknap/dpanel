package docker

import (
	"github.com/donknap/dpanel/common/service/exec"
	"github.com/donknap/dpanel/common/service/exec/local"
	exec2 "os/exec"
)

func (self Builder) Run(command ...string) (exec.Executor, error) {
	var cmd exec.Executor
	var err error
	options := make([]local.Option, 0)
	options = append(options, local.WithCommandName("docker"), local.WithArgs(append(
		self.DockerEnv.CommandParams(),
		command...,
	)...))
	cmd, err = local.New(options...)
	if err != nil {
		return nil, err
	}
	return cmd, nil
}

func (self Builder) Compose(command ...string) (exec.Executor, error) {
	var cmd exec.Executor
	var err error
	options := make([]local.Option, 0)
	if _, err := exec2.LookPath("docker-compose"); err == nil {
		options = append(options,
			local.WithCommandName("docker-compose"),
			local.WithArgs(command...),
			local.WithEnv(self.DockerEnv.CommandEnv()),
		)
	} else {
		options = append(options,
			local.WithCommandName("docker"),
			local.WithArgs(append(append(self.DockerEnv.CommandParams(), "compose"), command...)...),
		)
	}
	cmd, err = local.New(options...)
	if err != nil {
		return nil, err
	}
	return cmd, nil
}
