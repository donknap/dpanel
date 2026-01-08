package compose

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/compose-spec/compose-go/v2/types"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
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
	// dpanel 运行环境和远程 docker 可能环境不同，这里的命令统一采用相对目录的形式来处理
	// 如果当前是 windows 系统，则强制使用 /home/dpanel/ 做为数据目录
	cmd := make([]string, 0)
	cmd = append(cmd, "--project-directory", self.Project.WorkingDir)
	for _, file := range self.Project.ComposeFiles {
		if v, err := filepath.Rel(self.Project.WorkingDir, file); err == nil {
			cmd = append(cmd, "-f", v)
		}
	}
	cmd = append(cmd, "-p", self.Project.Name)
	for _, envFileName := range []string{
		".env",
	} {
		envFilePath := filepath.Join(self.Project.WorkingDir, envFileName)
		_, err := os.Stat(envFilePath)
		if err == nil {
			cmd = append(cmd, "--env-file", envFileName)
		}
	}
	cmd = append(cmd, command...)
	exec, err := docker.Sdk.Compose(cmd...)
	if err != nil {
		return nil, err
	}
	if self.Project != nil && self.Project.Environment != nil {
		exec.AppendEnv(function.PluckMapWalkArray(self.Project.Environment, func(k string, v string) (string, bool) {
			return fmt.Sprintf("%s=%s", k, v), true
		}))
	}
	exec.WorkDir(self.Project.WorkingDir)
	return exec.RunInPip()
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
