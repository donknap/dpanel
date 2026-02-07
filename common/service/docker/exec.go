package docker

import (
	exec2 "os/exec"
	"runtime"

	"github.com/donknap/dpanel/common/service/exec"
	"github.com/donknap/dpanel/common/service/exec/local"
)

func (self Client) Run(command ...string) (exec.Executor, error) {
	var cmd exec.Executor
	var err error
	options := make([]local.Option, 0)

	if runtime.GOOS == "windows" && command[0] == "buildx" {
		options = append(options,
			local.WithCommandName("docker-buildx"),
			local.WithArgs(command[1:]...),
			local.WithEnv(self.DockerEnv.CommandEnv()),
		)
	} else {
		options = append(options,
			local.WithCommandName("docker"),
			local.WithArgs(command...),
			local.WithEnv(self.DockerEnv.CommandEnv()),
		)
	}
	cmd, err = local.New(options...)
	if err != nil {
		return nil, err
	}
	return cmd, nil
}

func (self Client) RunResult(command ...string) ([]byte, error) {
	cmd, err := self.Run(command...)
	if err != nil {
		return nil, err
	}
	return cmd.RunWithResult()
}

func (self Client) Compose(command ...string) (exec.Executor, error) {
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
			local.WithEnv(self.DockerEnv.CommandEnv()),
		)
	}
	cmd, err = local.New(options...)
	if err != nil {
		return nil, err
	}
	return cmd, nil
}
