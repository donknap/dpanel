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
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// docker compose 任务执行，包含 部署，销毁，控制

type Task struct {
	Name     string
	Composer *Wrapper
	Status   string
}

func (self Task) Deploy(removeOrphans bool, pullImage bool) (io.ReadCloser, error) {
	cmd := []string{
		//"--progress", "tty",
		"up", "-d", "--build",
	}
	if pullImage {
		cmd = append(cmd, "--pull", "always")
	}
	if removeOrphans {
		cmd = append(cmd, "--remove-orphans")
	}

	if !function.IsEmptyArray(self.Project().DisabledServiceNames()) {
		cmd = append(cmd, self.Project().ServiceNames()...)
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

func (self Task) Build() (io.ReadCloser, error) {
	cmd := []string{
		//"--progress", "tty",
		"build",
	}
	return self.runCommand(cmd)
}

func (self Task) Destroy(deleteImage bool, deleteVolume bool) (io.ReadCloser, error) {
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

	if !function.IsEmptyArray(self.Project().DisabledServiceNames()) {
		cmd = append(cmd, self.Project().DisabledServiceNames()...)
	}

	return self.runCommand(cmd)
}

func (self Task) Ctrl(op string) (io.ReadCloser, error) {
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
	Project    string                      `json:"project"`
	Service    string                      `json:"service"`
	Publishers []ContainerPublishersResult `json:"publishers"`
	State      string                      `json:"state"`
	Status     string                      `json:"status"`
	Health     string
}

type ContainerPublishersResult struct {
	URL           string `json:"url"`
	TargetPort    uint16 `json:"targetPort"`
	PublishedPort uint16 `json:"publishedPort"`
	Protocol      string `json:"protocol"`
}

// Ps Deprecated /**
func (self Task) Ps() []*ContainerResult {
	result := make([]*ContainerResult, 0)
	if self.Name == "" {
		return result
	}
	// self.runCommand 只负责执行，Ps 命令需要返回结果
	args := self.getBaseCommand()
	args = append(args, "ps", "--format", "json", "--all")

	cmd, err := docker.Sdk.Compose(args...)
	if err != nil {
		return result
	}

	out, err := cmd.RunWithResult()
	if err != nil {
		return result
	}

	if strings.HasPrefix(string(out), "[{") {
		// 兼容 docker-compose ps 返回数据
		temp := make([]*ContainerResult, 0)
		err := json.Unmarshal(out, &temp)
		if err != nil {
			slog.Debug("compose task docker-compose failed", err.Error())
			return nil
		}
		return temp
	} else {
		newReader := bufio.NewReader(bytes.NewReader(out))
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
			if err = json.Unmarshal(line, &temp); err == nil {
				if temp.Health != "" {
					temp.State = temp.Health
				}
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
	command = append(self.getBaseCommand(), command...)
	cmd, err := docker.Sdk.Compose(command...)
	if err != nil {
		return nil, err
	}
	if self.Composer != nil && self.Composer.Project != nil && self.Composer.Project.Environment != nil {
		cmd.AppendEnv(function.PluckMapWalkArray(self.Composer.Project.Environment, func(k string, v string) (string, bool) {
			return fmt.Sprintf("%s=%s", k, v), true
		}))
	}
	return cmd.RunInPip()
}

func (self Task) getBaseCommand() []string {
	project := self.Project()
	cmd := make([]string, 0)
	for _, file := range self.Project().ComposeFiles {
		cmd = append(cmd, "-f", file)
	}
	cmd = append(cmd, "-p", project.Name)
	for _, envFileName := range []string{
		".env",
	} {
		envFilePath := filepath.Join(project.WorkingDir, envFileName)
		_, err := os.Stat(envFilePath)
		if err == nil {
			cmd = append(cmd, "--env-file", envFilePath)
		}
	}
	return cmd
}
