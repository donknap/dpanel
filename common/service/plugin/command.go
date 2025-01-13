package plugin

import (
	"bytes"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"io"
	"log/slog"
)

type Command struct {
}

// Result 执行一条命令返回结果，适用于查询查，防止两个command结果重复
func (self Command) Result(containerName string, cmd string) (string, error) {
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
	slog.Debug("command", "exec", []string{
		"/bin/sh",
		"-c",
		cmd,
	})
	response, err := self.Exec(containerName, execConfig)
	if err != nil {
		return "", err
	}
	defer response.Close()

	buffer := new(bytes.Buffer)
	_, err = io.Copy(buffer, response.Reader)
	if err != nil {
		return "", err
	}
	cleanOut := self.Clean(buffer.Bytes())
	slog.Debug("command", "clear result", cleanOut)
	return cleanOut, nil
}

func (self Command) Exec(containerName string, option container.ExecOptions) (types.HijackedResponse, error) {
	slog.Debug("docker exec", "command", option)
	exec, err := docker.Sdk.Client.ContainerExecCreate(docker.Sdk.Ctx, containerName, option)
	if err != nil {
		return types.HijackedResponse{}, err
	}
	execAttachOption := container.ExecStartOptions{
		Tty:         option.Tty,
		ConsoleSize: option.ConsoleSize,
		Detach:      option.Detach,
	}
	return docker.Sdk.Client.ContainerExecAttach(docker.Sdk.Ctx, exec.ID, execAttachOption)
}

func (self Command) Clean(str []byte) string {
	// 执行命令时返回的结果应该以 utf8 字符返回，并过滤掉不可见字符
	out := function.BytesCleanFunc(str, func(b byte) bool {
		return b < 32 && b != '\n' && b != '\r' && b != '\t'
	})
	return string(out)
}
