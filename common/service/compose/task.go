package compose

import (
	"encoding/json"
	"errors"
	"github.com/compose-spec/compose-go/v2/types"
	"github.com/docker/docker/api/types/network"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/exec"
	"os"
	"strings"
)

// docker compose 任务执行，包含 部署，销毁，控制

func NewTasker(name string, wrapper *Wrapper) *Task {
	return &Task{
		Name:     name,
		composer: wrapper,
	}
}

type Task struct {
	Name     string
	composer *Wrapper
}

func (self Task) Deploy() error {
	cmd := []string{
		"--progress", "tty", "up", "-d",
	}
	cmd = append(cmd, self.composer.GetServiceNameList()...)
	self.runCommand(cmd)

	// 如果 compose 中未指定网络，则默认的名称为 项目名_default
	for _, item := range self.composer.Project.Networks {
		for _, serviceItem := range self.composer.Project.Services {
			for _, linkItem := range serviceItem.ExternalLinks {
				links := strings.Split(linkItem, ":")
				if len(links) == 2 {
					_ = docker.Sdk.Client.NetworkConnect(docker.Sdk.Ctx, item.Name, links[0], &network.EndpointSettings{})
				}
			}
		}
	}
	return nil
}

func (self Task) Destroy(deleteImage bool) error {
	cmd := []string{
		"--progress", "tty", "down",
	}
	// todo 删除compose 前需要先把关联的已有容器网络退出

	if deleteImage {
		cmd = append(cmd, "--rmi", "all")
	}
	self.runCommand(cmd)
	return nil
}

func (self Task) Ctrl(op string) error {
	cmd := []string{
		"--progress", "tty", op,
	}
	self.runCommand(cmd)
	return nil
}

func (self Task) Yaml() ([]byte, error) {
	if len(self.composer.Project.ComposeFiles) >= 1 {
		content, err := os.ReadFile(self.composer.Project.ComposeFiles[0])
		if err != nil {
			return nil, err
		}
		return content, nil
	}
	return nil, errors.New("compose yaml not found")
}

func (self Task) Project() *types.Project {
	return self.composer.Project
}

type composeContainerResult struct {
	Name string `json:"name"`
}

func (self Task) Ps() []*composeContainerResult {
	result := make([]*composeContainerResult, 0)
	if self.Name == "" {
		return result
	}
	// self.runCommand 只负责执行，Ps 命令需要返回结果
	cmd := self.composer.GetBaseCommand()
	cmd = append(cmd, "ps", "--format", "json", "--all")

	out := exec.Command{}.RunWithOut(&exec.RunCommandOption{
		CmdName: "docker",
		CmdArgs: append(append(docker.Sdk.ExtraParams, "compose"), cmd...),
	})
	if out == "" {
		return result
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "{") {
			temp := composeContainerResult{}
			err := json.Unmarshal([]byte(line), &temp)
			if err == nil {
				result = append(result, &temp)
			}
		}
	}
	return result
}

func (self Task) runCommand(command []string) {
	command = append(self.composer.GetBaseCommand(), command...)
	exec.Command{}.RunInTerminal(&exec.RunCommandOption{
		CmdName: "docker",
		CmdArgs: append(
			append(docker.Sdk.ExtraParams, "compose"),
			command...,
		),
	})
}
