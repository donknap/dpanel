package plugin

import (
	"github.com/docker/docker/api/types/container"
	"github.com/donknap/dpanel/common/service/docker"
)

type Command struct {
}

// Result 执行一条命令返回结果，适用于查询查，防止两个command结果重复
func (self Command) Result(containerName string, cmd string) (out []byte, err error) {
	execConfig := container.ExecOptions{
		Privileged:   true,
		Tty:          false,
		AttachStdin:  false,
		AttachStdout: true,
		AttachStderr: false,
		Cmd: []string{
			"/bin/sh",
			"-c",
			cmd,
		},
	}
	exec, err := docker.Sdk.Client.ContainerExecCreate(docker.Sdk.Ctx, containerName, execConfig)
	if err != nil {
		return out, err
	}
	o := &Hijacked{
		Id: exec.ID,
	}
	o.conn, err = docker.Sdk.Client.ContainerExecAttach(docker.Sdk.Ctx, exec.ID, container.ExecStartOptions{
		Tty: false,
	})
	defer o.Close()
	return o.Out(), nil
}
