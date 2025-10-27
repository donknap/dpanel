package docker

import (
	"fmt"
	"os"
	exec2 "os/exec"

	"github.com/donknap/dpanel/common/service/exec"
	"github.com/donknap/dpanel/common/service/exec/local"
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
			local.WithEnv(os.Environ()),
		)
	}
	cmd, err = local.New(options...)
	if err != nil {
		return nil, err
	}
	return cmd, nil
}

// CmdProxy Proxy 如果是 ssh 连接运行命令时需要先创建代理 sock
// 执行完成命令后，需要将整个上下文关闭
func (self Builder) CmdProxy() (*Builder, error) {
	return NewBuilderWithDockerEnv(&Client{
		Name:              fmt.Sprintf("%s-%s", self.DockerEnv.Name, "proxy"),
		Title:             self.DockerEnv.Title,
		Address:           self.DockerEnv.Address,
		Default:           false,
		DockerInfo:        self.DockerEnv.DockerInfo,
		ServerUrl:         self.DockerEnv.ServerUrl,
		EnableTLS:         self.DockerEnv.EnableTLS,
		TlsCa:             self.DockerEnv.TlsCa,
		TlsCert:           self.DockerEnv.TlsCert,
		TlsKey:            self.DockerEnv.TlsKey,
		EnableComposePath: self.DockerEnv.EnableComposePath,
		ComposePath:       self.DockerEnv.ComposePath,
		EnableSSH:         self.DockerEnv.EnableSSH,
		SshServerInfo:     self.DockerEnv.SshServerInfo,
		RemoteType:        self.DockerEnv.RemoteType,
	})
}
