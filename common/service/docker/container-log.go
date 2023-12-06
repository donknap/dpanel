package docker

import (
	"context"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"io"
	"strconv"
)

type HostInfo struct {
	EndpointID     int    `json:"EndpointID"`
	RawOutput      string `json:"RawOutput"`
	AMT            string `json:"AMT"`
	UUID           string `json:"UUID"`
	DNSSuffix      string `json:"DNS Suffix"`
	BuildNumber    string `json:"Build Number"`
	ControlMode    string `json:"Control Mode"`
	ControlModeRaw int    `json:"Control Mode (Raw)"`
}

type containerLogBuilder struct {
	option      types.ContainerLogsOptions
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
