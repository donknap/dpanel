package plugin

import (
	"github.com/docker/docker/api/types/container"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
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
	exec, err := docker.Sdk.Client.ContainerExecCreate(docker.Sdk.Ctx, containerName, execConfig)
	if err != nil {
		return "", err
	}
	o := &Hijacked{
		Id: exec.ID,
	}
	o.conn, err = docker.Sdk.Client.ContainerExecAttach(docker.Sdk.Ctx, exec.ID, container.ExecStartOptions{
		Tty: false,
	})
	defer o.Close()

	cleanOut := self.Clean(o.Out())
	slog.Debug("command", "result", cleanOut)
	return cleanOut, nil
}

func (self Command) Clean(str []byte) string {
	// 执行命令时返回的结果应该以 utf8 字符返回，并过滤掉不可见字符
	out := function.BytesCleanFunc(str, func(b byte) bool {
		return b < 32 && b != '\n' && b != '\r' && b != '\t'
	})
	utf8Out := function.BytesCleanFunc([]rune(string(out)), func(b rune) bool {
		return b == '\ufffd'
	})
	return string(utf8Out)
}
