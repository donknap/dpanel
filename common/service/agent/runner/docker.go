package runner

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
	serviceDocker "github.com/donknap/dpanel/common/service/docker"
)

const dockerExecutable = "/agent"

type Docker struct {
	dockerSdk     *serviceDocker.Client
	containerName string
}

func NewDocker(dockerSdk *serviceDocker.Client, containerName string) (*Docker, error) {
	if dockerSdk == nil || dockerSdk.Client == nil {
		return nil, errors.New("docker client is required for agent runner")
	}
	if containerName == "" {
		return nil, errors.New("container name is required for agent runner")
	}
	return &Docker{dockerSdk: dockerSdk, containerName: containerName}, nil
}

func (self *Docker) Run(ctx context.Context, args ...string) ([]byte, error) {
	command := append([]string{dockerExecutable}, args...)
	output, err := self.dockerSdk.ContainerExecResult(ctx, self.containerName, container.ExecOptions{Cmd: command})
	return []byte(output), err
}

func (self *Docker) Stream(ctx context.Context, args ...string) (Session, error) {
	command := append([]string{dockerExecutable}, args...)
	_, response, err := self.dockerSdk.ContainerExec(ctx, self.containerName, container.ExecOptions{
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          command,
	})
	if err != nil {
		return nil, err
	}
	stdoutReader, stdoutWriter := io.Pipe()
	result := &dockerSession{response: response, stdout: stdoutReader}
	go func() {
		var stderr bytes.Buffer
		_, err := stdcopy.StdCopy(stdoutWriter, &stderr, response.Reader)
		if err == nil && stderr.Len() > 0 {
			err = errors.New(stderr.String())
		}
		_ = stdoutWriter.CloseWithError(err)
	}()
	return result, nil
}

type dockerSession struct {
	response types.HijackedResponse
	stdout   *io.PipeReader
	writeMu  sync.Mutex
	closeMu  sync.Once
}

func (self *dockerSession) Read(data []byte) (int, error) {
	return self.stdout.Read(data)
}

func (self *dockerSession) Write(data []byte) (int, error) {
	self.writeMu.Lock()
	defer self.writeMu.Unlock()
	return self.response.Conn.Write(data)
}

func (self *dockerSession) CloseWrite() error {
	return self.response.CloseWrite()
}

func (self *dockerSession) Close() error {
	self.closeMu.Do(func() {
		_ = self.stdout.Close()
		self.response.Close()
	})
	return nil
}
