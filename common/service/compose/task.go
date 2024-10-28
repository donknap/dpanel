package compose

import (
	"encoding/json"
	"github.com/compose-spec/compose-go/v2/types"
	"github.com/docker/docker/api/types/network"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/exec"
	"strings"
)

// docker compose 任务执行，包含 部署，销毁，控制

type Task struct {
	Name     string
	Composer *Wrapper
	Original *Wrapper // 原始 compose
}

func (self Task) Deploy() error {
	cmd := []string{
		"--progress", "tty", "up", "-d",
	}
	cmd = append(cmd, self.Composer.GetServiceNameList()...)
	self.runCommand(cmd)

	for _, item := range self.Composer.Project.Networks {
		for _, serviceItem := range self.Composer.Project.Services {
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

func (self Task) Destroy(deleteImage bool, deleteVolume bool) error {
	cmd := []string{
		"--progress", "tty", "down",
	}
	// 删除compose 前需要先把关联的已有容器网络退出
	for _, item := range self.Composer.Project.Networks {
		for _, serviceItem := range self.Composer.Project.Services {
			for _, linkItem := range serviceItem.ExternalLinks {
				links := strings.Split(linkItem, ":")
				if len(links) == 2 {
					_ = docker.Sdk.Client.NetworkDisconnect(docker.Sdk.Ctx, item.Name, links[0], true)
				}
			}
		}
	}

	if deleteImage {
		cmd = append(cmd, "--rmi", "all")
	}

	if deleteVolume {
		cmd = append(cmd, "--volumes")
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

func (self Task) OriginalYaml() ([]byte, error) {
	return self.Original.Project.MarshalYAML()
}

func (self Task) Project() *types.Project {
	return self.Composer.Project
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
	cmd := self.Composer.GetBaseCommand()
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
	command = append(self.Composer.GetBaseCommand(), command...)
	exec.Command{}.RunInTerminal(&exec.RunCommandOption{
		CmdName: "docker",
		CmdArgs: append(
			append(docker.Sdk.ExtraParams, "compose"),
			command...,
		),
	})
}
