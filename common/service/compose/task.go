package compose

import (
	"encoding/json"
	"fmt"
	"github.com/compose-spec/compose-go/v2/types"
	"github.com/docker/docker/api/types/network"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/opencontainers/go-digest"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Task struct {
	Name    string
	Project *types.Project
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

	if !function.IsEmptyArray(self.Project.DisabledServiceNames()) {
		cmd = append(cmd, self.Project.ServiceNames()...)
	}

	response, err := self.runCommand(cmd)
	if err != nil {
		return nil, err
	}

	for _, item := range self.Project.Networks {
		for _, serviceItem := range self.Project.Services {
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
	for _, item := range self.Project.Networks {
		for _, serviceItem := range self.Project.Services {
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

	if !function.IsEmptyArray(self.Project.DisabledServiceNames()) {
		cmd = append(cmd, self.Project.DisabledServiceNames()...)
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

func (self Task) runCommand(command []string) (io.ReadCloser, error) {
	command = append(self.getBaseCommand(), command...)
	cmd, err := docker.Sdk.Compose(command...)
	if err != nil {
		return nil, err
	}
	if self.Project != nil && self.Project.Environment != nil {
		cmd.AppendEnv(function.PluckMapWalkArray(self.Project.Environment, func(k string, v string) (string, bool) {
			return fmt.Sprintf("%s=%s", k, v), true
		}))
	}
	return cmd.RunInPip()
}

func (self Task) getBaseCommand() []string {
	project := self.Project
	cmd := make([]string, 0)
	for _, file := range self.Project.ComposeFiles {
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

// GetService 区别于 Project.GetService 方法，此方法会将扩展信息一起返回
func (self Task) GetService(name string) (types.ServiceConfig, ExtService, error) {
	service, err := self.Project.GetService(name)
	if err != nil {
		return types.ServiceConfig{}, ExtService{}, err
	}

	ext := ExtService{}
	exists, err := service.Extensions.Get(ExtensionServiceName, &ext)
	if err == nil && exists {
		return service, ext, nil
	}
	return service, ExtService{}, nil
}

func (self Task) GetServiceConfigHash(serviceName string) (string, error) {
	o, err := self.Project.GetService(serviceName)
	if err != nil {
		return "", err
	}
	// remove the Build config when generating the service hash
	o.Build = nil
	o.PullPolicy = ""
	o.Scale = nil
	if o.Deploy != nil {
		o.Deploy.Replicas = nil
	}
	o.DependsOn = nil
	o.Profiles = nil

	bytes, err := json.Marshal(o)
	if err != nil {
		return "", err
	}
	return digest.SHA256.FromBytes(bytes).Encoded(), nil
}
