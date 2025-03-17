package docker

import (
	"bytes"
	"github.com/docker/docker/api/types/container"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/exec"
	"io"
	"log/slog"
	exec2 "os/exec"
)

func (self Builder) GetRunCmd(command ...string) []exec.Option {
	return []exec.Option{
		exec.WithCommandName("docker"),
		exec.WithArgs(append(
			self.runParams,
			command...,
		)...),
	}
}

func (self Builder) GetComposeCmd(command ...string) []exec.Option {
	if _, err := exec2.LookPath("docker-compose"); err == nil {
		return []exec.Option{
			exec.WithCommandName("docker-compose"),
			exec.WithArgs(command...),
			exec.WithEnv(self.runEnv),
		}
	} else {
		return []exec.Option{
			exec.WithCommandName("docker"),
			exec.WithArgs(append(append(self.runParams, "compose"), command...)...),
		}
	}
}

// ExecResult 在容器中执行一条命令，返回结果
func (self Builder) ExecResult(containerName string, cmd string) (string, error) {
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
	response, err := Sdk.ContainerExec(containerName, execConfig)
	if err != nil {
		return "", err
	}
	defer response.Close()

	buffer := new(bytes.Buffer)
	_, err = io.Copy(buffer, response.Reader)
	if err != nil {
		return "", err
	}
	cleanOut := self.ExecCleanResult(buffer.Bytes())
	slog.Debug("command", "clear result", cleanOut)
	return cleanOut, nil
}

func (self Builder) ExecCleanResult(str []byte) string {
	// 执行命令时返回的结果应该以 utf8 字符返回，并过滤掉不可见字符
	out := function.BytesCleanFunc(str, func(b byte) bool {
		return b < 32 && b != '\n' && b != '\r' && b != '\t'
	})
	return string(out)
}
