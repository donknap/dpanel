package docker

import (
	"context"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"io"
	"strconv"
)

type containerLogBuilder struct {
	option      container.LogsOptions
	dockerSdk   *client.Client
	ctx         context.Context
	containerId string
}

func (self *containerLogBuilder) withSdk(sdk *client.Client) {
	self.dockerSdk = sdk
}

func (self *containerLogBuilder) WithContainerId(id string) {
	self.containerId = id
}

func (self *containerLogBuilder) WithTail(line int) {
	if line == 0 {
		self.option.Tail = "all"
	} else {
		self.option.Tail = strconv.Itoa(line)
	}
}

func (self *containerLogBuilder) WithShowType(showStdOut bool, showStdErr bool) {
	self.option.ShowStderr = showStdErr
	self.option.ShowStdout = showStdOut
}

func (self *containerLogBuilder) Execute() (result string, err error) {
	out, err := self.dockerSdk.ContainerLogs(context.Background(), self.containerId, self.option)
	if err != nil {
		return result, err
	}
	output, err := io.ReadAll(out)
	if err != nil {
		return result, err
	}
	return string(output), nil
}
