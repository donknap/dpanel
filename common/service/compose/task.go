package compose

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/compose-spec/compose-go/v2/types"
	"github.com/docker/docker/api/types/network"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/exec"
	"io"
	"log/slog"
	"os"
	"strings"
)

// docker compose 任务执行，包含 部署，销毁，控制

type Task struct {
	Name     string
	Composer *Wrapper
	Status   string
}

func (self Task) Deploy(serviceName []string, removeOrphans bool) (io.Reader, error) {
	cmd := []string{
		//"--progress", "tty",
		"up", "-d",
	}

	if removeOrphans {
		cmd = append(cmd, "--remove-orphans")
	}

	if !function.IsEmptyArray(serviceName) {
		cmd = append(cmd, serviceName...)
	}

	response, err := self.runCommand(cmd)
	if err != nil {
		return nil, err
	}
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
	return response, nil
}

func (self Task) Destroy(deleteImage bool, deleteVolume bool) (io.Reader, error) {
	cmd := []string{
		//"--progress", "tty",
		"down", "--remove-orphans",
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
	return self.runCommand(cmd)
}

func (self Task) Ctrl(op string) (io.Reader, error) {
	cmd := []string{
		//"--progress", "tty",
		op,
	}
	return self.runCommand(cmd)
}

func (self Task) Logs(tail int, showTime, follow bool) (io.ReadCloser, error) {
	cmd := []string{
		//"--progress", "tty",
		"logs",
	}
	if tail > 0 {
		cmd = append(cmd, "--tail", fmt.Sprintf("%d", tail))
	}
	if showTime {
		cmd = append(cmd, "-t")
	}
	if follow {
		cmd = append(cmd, "-f")
	}
	return self.runCommand(cmd)
}

func (self Task) Project() *types.Project {
	return self.Composer.Project
}

type ContainerResult struct {
	Name       string                      `json:"name"`
	Service    string                      `json:"service"`
	Publishers []ContainerPublishersResult `json:"publishers"`
	State      string                      `json:"state"`
	Status     string                      `json:"status"`
}

type ContainerPublishersResult struct {
	URL           string `json:"url"`
	TargetPort    uint16 `json:"targetPort"`
	PublishedPort uint16 `json:"publishedPort"`
	Protocol      string `json:"protocol"`
}

func (self Task) Ps() []*ContainerResult {
	result := make([]*ContainerResult, 0)
	if self.Name == "" {
		return result
	}
	// self.runCommand 只负责执行，Ps 命令需要返回结果
	args := self.Composer.GetBaseCommand()
	args = append(args, "ps", "--format", "json", "--all")

	cmd, err := exec.New(docker.Sdk.GetComposeCmd(args...)...)
	if err != nil {
		return result
	}

	out := cmd.RunWithResult()
	if out == "" {
		return result
	}

	if strings.HasPrefix(out, "[{") {
		// 兼容 docker-compose ps 返回数据
		temp := make([]*ContainerResult, 0)
		err := json.Unmarshal([]byte(out), &temp)
		if err != nil {
			slog.Debug("compose task docker-compose failed", err.Error())
			return nil
		}
		return temp
	} else {
		newReader := bufio.NewReader(bytes.NewReader([]byte(out)))
		line := make([]byte, 0)
		for {
			t, isPrefix, err := newReader.ReadLine()
			if err == io.EOF {
				break
			}
			line = append(line, t...)
			if isPrefix {
				continue
			}
			temp := ContainerResult{}
			err = json.Unmarshal(line, &temp)
			if err == nil {
				result = append(result, &temp)
			}
			line = make([]byte, 0)
		}
		return result
	}
}

func (self Task) GetYaml() ([2]string, error) {
	yaml := [2]string{
		"", "",
	}
	for i, uri := range self.Project().ComposeFiles {
		content, err := os.ReadFile(uri)
		if err == nil {
			yaml[i] = string(content)
		}
	}
	return yaml, nil
}

func (self Task) runCommand(command []string) (io.ReadCloser, error) {
	command = append(self.Composer.GetBaseCommand(), command...)
	cmd, err := exec.New(docker.Sdk.GetComposeCmd(command...)...)
	if err != nil {
		return nil, err
	}
	return cmd.RunInPip()
}
