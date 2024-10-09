package logic

import (
	"encoding/json"
	"github.com/docker/docker/api/types/network"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/exec"
	"github.com/donknap/dpanel/common/service/storage"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const (
	ComposeTypeText        = "text"
	ComposeTypeRemoteUrl   = "remoteUrl"
	ComposeTypeServerPath  = "serverPath"
	ComposeTypeStoragePath = "storagePath"
)

type ComposeTaskOption struct {
	Name        string
	Yaml        string
	Environment []accessor.EnvItem
	DeleteImage bool
}

type composeItem struct {
	Name        string `json:"name"`
	Status      string `json:"status"`
	ConfigFiles string `json:"configFiles"`
}

type composeContainerItem struct {
	Name string `json:"name"`
}

type Compose struct {
}

func (self Compose) Deploy(task *ComposeTaskOption) error {
	envFile, err := self.getEnvFile(task.Environment)
	if err != nil {
		return err
	}
	defer os.Remove(envFile.Name())

	yamlFile, err := self.getYamlFile(task.Yaml)
	if err != nil {
		return err
	}
	defer os.Remove(yamlFile.Name())

	dockerYaml, err := docker.NewYaml(task.Yaml)
	if err != nil {
		return err
	}
	self.runCommand(append([]string{
		"-f", yamlFile.Name(),
		"-p", task.Name,
		"--env-file", envFile.Name(),
		"--progress", "tty",
		"up",
		"-d",
	}, dockerYaml.GetServiceName()...))

	// 部署完成后还需要把外部容器加入到对应的网络中
	// 如果 compose 中未指定网络，则默认的名称为 项目名_default
	for _, item := range dockerYaml.GetExternalLinks() {
		for _, name := range dockerYaml.GetNetworkList() {
			err = docker.Sdk.Client.NetworkConnect(docker.Sdk.Ctx, task.Name+"_"+name, item.ContainerName, &network.EndpointSettings{})
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (self Compose) Destroy(task *ComposeTaskOption) error {
	dockerYaml, err := docker.NewYaml(task.Yaml)
	if err != nil {
		return err
	}
	// 删除compose 前需要先把关联的已有容器网络退出
	for _, item := range dockerYaml.GetExternalLinks() {
		for _, name := range dockerYaml.GetNetworkList() {
			err = docker.Sdk.Client.NetworkDisconnect(docker.Sdk.Ctx, task.Name+"_"+name, item.ContainerName, true)
			if err != nil {
				return err
			}
		}
	}

	envFile, err := self.getEnvFile(task.Environment)
	if err != nil {
		return err
	}
	defer os.Remove(envFile.Name())

	yamlFile, err := self.getYamlFile(task.Yaml)
	if err != nil {
		return err
	}
	defer os.Remove(yamlFile.Name())

	command := []string{
		"-f", yamlFile.Name(),
		"-p", task.Name,
		"--env-file", envFile.Name(),
		"--progress",
		"tty",
		"down",
	}
	if task.DeleteImage {
		command = append(command, "--rmi", "all")
	}
	self.runCommand(command)
	return nil
}

func (self Compose) Ctrl(task *ComposeTaskOption, op string) error {
	envFile, err := self.getEnvFile(task.Environment)
	if err != nil {
		return err
	}
	defer os.Remove(envFile.Name())

	yamlFile, err := self.getYamlFile(task.Yaml)
	if err != nil {
		return err
	}
	command := []string{
		"-f", yamlFile.Name(),
		"-p", task.Name,
		"--env-file", envFile.Name(),
		"--progress", "tty",
		op,
	}
	self.runCommand(command)
	os.Remove(yamlFile.Name())
	return nil
}

func (self Compose) Ls(projectName string) []*composeItem {
	command := []string{
		"ls",
		"--format", "json",
		"--all",
	}
	if projectName != "" {
		command = append(command, "--filter", "name="+projectName)
	}
	out := exec.Command{}.RunWithOut(&exec.RunCommandOption{
		CmdName: "docker",
		CmdArgs: append(append(docker.Sdk.ExtraParams, "compose"), command...),
	})
	result := make([]*composeItem, 0)
	err := json.Unmarshal([]byte(out), &result)
	if err != nil {
		return result
	}
	return result
}

func (self Compose) Ps(task *ComposeTaskOption) []*composeContainerItem {
	result := make([]*composeContainerItem, 0)
	if task.Name == "" || task.Yaml == "" {
		return result
	}

	yamlFile, err := self.getYamlFile(task.Yaml)
	if err != nil {
		return result
	}
	defer os.Remove(yamlFile.Name())

	command := []string{
		"-f", yamlFile.Name(),
		"-p", task.Name,
		"ps",
		"--format", "json",
		"--all",
	}
	out := exec.Command{}.RunWithOut(&exec.RunCommandOption{
		CmdName: "docker",
		CmdArgs: append(append(docker.Sdk.ExtraParams, "compose"), command...),
	})

	if out == "" {
		return result
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "{") {
			temp := composeContainerItem{}
			err = json.Unmarshal([]byte(line), &temp)
			if err == nil {
				result = append(result, &temp)
			}
		}
	}
	return result
}

func (self Compose) Kill() error {
	return exec.Command{}.Kill()
}

func (self Compose) runCommand(command []string) {
	exec.Command{}.RunInTerminal(&exec.RunCommandOption{
		CmdName: "docker",
		CmdArgs: append(
			append(docker.Sdk.ExtraParams, "compose"),
			command...,
		),
	})
}

func (self Compose) getYamlFile(yamlContent string) (*os.File, error) {
	yamlFile, _ := os.CreateTemp("", "dpanel-compose")
	err := os.WriteFile(yamlFile.Name(), []byte(yamlContent), 0666)
	if err != nil {
		return nil, err
	}
	return yamlFile, nil
}

func (self Compose) getEnvFile(env []accessor.EnvItem) (*os.File, error) {
	envFile, _ := os.CreateTemp("", "dpanel-compose-env")
	envContent := make([]string, 0)
	for _, item := range env {
		envContent = append(envContent, item.Name+"="+item.Value)
	}
	err := os.WriteFile(envFile.Name(), []byte(strings.Join(envContent, "\n")), 0666)
	if err != nil {
		return nil, err
	}
	return envFile, nil
}

func (self Compose) Sync() error {
	composeList, _ := dao.Compose.Find()

	composeFileName := []string{
		"docker-compose.yml", "docker-compose.yaml",
		"compose.yml", "compose.yaml",
	}
	rootDir := storage.Local{}.GetComposePath()
	filepath.Walk(rootDir, func(path string, info fs.FileInfo, err error) error {
		for _, suffix := range composeFileName {
			if strings.HasSuffix(path, suffix) {
				rel, _ := filepath.Rel(rootDir, path)
				// 只同步二级目录下的 yaml
				if segments := strings.Split(filepath.Clean(rel), string(filepath.Separator)); len(segments) == 2 {
					name := filepath.Dir(rel)

					has := false
					for _, item := range composeList {
						if item.Name == name {
							has = true
							break
						}
					}

					if !has {
						dao.Compose.Create(&entity.Compose{
							Title: "",
							Name:  name,
							Yaml:  rel,
							Setting: &accessor.ComposeSettingOption{
								Type:   ComposeTypeStoragePath,
								Status: "waiting",
							},
						})
					}
				}
				break
			}
		}
		return nil
	})
	return nil
}
