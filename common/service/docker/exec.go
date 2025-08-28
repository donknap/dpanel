package docker

import (
	"github.com/donknap/dpanel/common/service/exec"
	"github.com/donknap/dpanel/common/service/exec/local"
	exec2 "os/exec"
	"runtime"
)

func (self Builder) Run(command ...string) (exec.Executor, error) {
	var cmd exec.Executor
	var err error
	options := make([]local.Option, 0)
	options = append(options, local.WithCommandName("docker"), local.WithArgs(append(
		self.DockerEnv.CommandParams(),
		command...,
	)...))
	if CliInWSL() {
		args := append(
			[]string{
				"docker",
			},
			self.DockerEnv.CommandParams()...,
		)
		options = []local.Option{
			local.WithCommandName("wsl"),
			local.WithArgs(append(args, command...)...),
		}
	}
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
	if CliInWSL() {
		args := append(
			[]string{
				"docker",
			},
			self.DockerEnv.CommandParams()...,
		)
		args = append(args, "compose")
		args = append(args, command...)
		options = append(options,
			local.WithCommandName("wsl"),
			local.WithArgs(args...),
		)
	} else {
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
	}
	cmd, err = local.New(options...)
	if err != nil {
		return nil, err
	}
	return cmd, nil
}

func CliInWSL() bool {
	if runtime.GOOS != "windows" {
		return false
	}
	if _, err := exec2.LookPath("docker"); err == nil {
		return false
	}
	if _, err := local.QuickRun("wsl docker version"); err == nil {
		return true
	}
	return false
}
