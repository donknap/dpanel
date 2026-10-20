package compose

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/compose-spec/compose-go/v2/paths"
	"github.com/compose-spec/compose-go/v2/types"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	types2 "github.com/donknap/dpanel/common/service/docker/types"
	"github.com/donknap/dpanel/common/types/define"
)

type Task struct {
	dockerSdk    *docker.Client
	Name         string
	Project      *types.Project
	ComposeFiles []string
	WorkingDir   string
	Environment  []types2.EnvItem
}

func (self Task) GetYaml() [2]string {
	yaml := [2]string{}
	for index, composeFile := range self.ComposeFiles {
		if index >= len(yaml) {
			break
		}
		if content, err := os.ReadFile(composeFile); err == nil {
			yaml[index] = string(content)
		}
	}
	return yaml
}

func (self Task) GetDefaultEnv() (envFile string, envFileContent []byte, err error) {
	envFile = filepath.Join(filepath.Dir(self.ComposeFiles[0]), define.ComposeDefaultEnvFileName)
	envFileContent, err = os.ReadFile(envFile)
	if errors.Is(err, os.ErrNotExist) {
		return envFile, nil, nil
	}
	return envFile, envFileContent, err
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

func (self Task) CreateBindPath() {
	if self.dockerSdk.DockerEnv.IsRemote() {
		return
	}
	for _, service := range self.Project.Services {
		for _, volume := range service.Volumes {
			if volume.Type != types.VolumeTypeBind || volume.Source == "" {
				continue
			}
			source := paths.ExpandUser(volume.Source)
			if !filepath.IsAbs(source) {
				if _, windowsPath := function.WindowsPathToSlash(source); !windowsPath {
					source = filepath.Join(function.SlashPathToSystem(self.WorkingDir), source)
				}
			}
			if converted, ok := function.WindowsPathToSlash(source); ok && function.IsRunInDocker() {
				source = filepath.Join("/mnt/host", strings.TrimPrefix(converted, "/"))
			}
			if filepath.IsAbs(source) {
				if err := os.MkdirAll(source, os.ModePerm); err != nil {
					slog.Warn("compose create bind path", "path", source, "error", err)
				}
			}
		}
	}
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
					_ = self.dockerSdk.Client.NetworkDisconnect(self.dockerSdk.Ctx, item.Name, links[0], true)
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
	sourceDir := filepath.Dir(self.ComposeFiles[0])
	workingDir := function.SlashPathToSystem(self.WorkingDir)
	if hostDir, ok := function.WindowsPathToSlash(self.WorkingDir); ok && function.IsRunInDocker() {
		workingDir = filepath.Join("/mnt/host", strings.TrimPrefix(hostDir, "/"))
	}
	cmd := make([]string, 0)
	cmd = append(cmd, "--project-directory", workingDir)
	for _, file := range self.ComposeFiles {
		cmd = append(cmd, "-f", file)
	}
	cmd = append(cmd, "-p", self.Project.Name)
	for _, envFileName := range []string{
		".env",
	} {
		envFilePath := filepath.Join(sourceDir, envFileName)
		_, err := os.Stat(envFilePath)
		if err == nil {
			cmd = append(cmd, "--env-file", envFilePath)
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}
	cmd = append(cmd, command...)
	exec, err := self.dockerSdk.Compose(cmd...)
	if err != nil {
		return nil, err
	}
	if self.Project.Environment != nil {
		exec.AppendEnv(function.PluckMapWalkArray(self.Project.Environment, func(k string, v string) (string, bool) {
			return fmt.Sprintf("%s=%s", k, v), true
		}))
	}
	exec.WorkDir(sourceDir)
	return exec.RunInReadPip()
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
