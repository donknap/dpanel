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
	"io"
	"io/fs"
	"net/http"
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
	Entity      *entity.Compose
	DeleteImage bool
}

func (self ComposeTaskOption) getEnvFile() (string, error) {
	envFile, _ := os.CreateTemp("", "dpanel-compose-env")
	defer func() {
		_ = envFile.Close()
	}()
	envContent := make([]string, 0)
	for _, item := range self.Entity.Setting.Environment {
		envContent = append(envContent, item.Name+"="+item.Value)
	}
	err := os.WriteFile(envFile.Name(), []byte(strings.Join(envContent, "\n")), 0666)
	if err != nil {
		return "", err
	}
	return envFile.Name(), nil
}

func (self ComposeTaskOption) getYamlFile() (path string, hasDelete bool, err error) {
	path = ""
	hasDelete = false
	err = nil

	if self.Entity.Setting.Type == ComposeTypeServerPath {
		path = self.Entity.Yaml
		hasDelete = false
		return
	}

	if self.Entity.Setting.Type == ComposeTypeStoragePath {
		path = filepath.Join(storage.Local{}.GetComposePath(), self.Entity.Yaml)
		hasDelete = false
		return
	}

	yamlFile, _ := os.CreateTemp("", "dpanel-compose")
	defer func() {
		_ = yamlFile.Close()
	}()

	if self.Entity.Setting.Type == ComposeTypeText || self.Entity.Setting.Type == "" {
		err = os.WriteFile(yamlFile.Name(), []byte(self.Entity.Yaml), 0666)
		if err != nil {
			return "", hasDelete, err
		}
	}

	if self.Entity.Setting.Type == ComposeTypeRemoteUrl {
		var response *http.Response
		var content []byte

		response, err = http.Get(self.Entity.Yaml)
		if err != nil {
			return
		}
		defer func() {
			_ = response.Body.Close()
		}()
		content, err = io.ReadAll(response.Body)
		if err != nil {
			return
		}
		err = os.WriteFile(yamlFile.Name(), []byte(content), 0666)
		if err != nil {
			return
		}
	}
	path = yamlFile.Name()
	hasDelete = true
	return
}

func (self ComposeTaskOption) GetYaml() (string, error) {
	if self.Entity.Setting.Type == ComposeTypeText || self.Entity.Setting.Type == "" {
		return self.Entity.Yaml, nil
	}
	yamlFilePath, hasDelete, err := self.getYamlFile()
	if err != nil {
		return "", err
	}
	if hasDelete {
		defer os.Remove(yamlFilePath)
	}
	content, err := os.ReadFile(yamlFilePath)
	if err != nil {
		return "", err
	}
	return string(content), nil
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
	envFile, err := task.getEnvFile()
	if err != nil {
		return err
	}
	defer os.Remove(envFile)

	yamlFilePath, hasDelete, err := task.getYamlFile()
	if err != nil {
		return err
	}
	if hasDelete {
		// 只有是系统生成的临时yaml文件才删除
		defer os.Remove(yamlFilePath)
	}

	yamlContent, err := os.ReadFile(yamlFilePath)
	dockerYaml, err := docker.NewYaml(yamlContent)
	if err != nil {
		return err
	}

	self.runCommand(append([]string{
		"-f", yamlFilePath,
		"-p", task.Entity.Name,
		"--env-file", envFile,
		"--progress", "tty",
		"up",
		"-d",
	}, dockerYaml.GetServiceName()...))

	// 部署完成后还需要把外部容器加入到对应的网络中
	// 如果 compose 中未指定网络，则默认的名称为 项目名_default
	for _, item := range dockerYaml.GetExternalLinks() {
		for _, name := range dockerYaml.GetNetworkList() {
			err = docker.Sdk.Client.NetworkConnect(docker.Sdk.Ctx, task.Entity.Name+"_"+name, item.ContainerName, &network.EndpointSettings{})
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (self Compose) Destroy(task *ComposeTaskOption) error {
	yamlFilePath, hasDelete, err := task.getYamlFile()
	if err != nil {
		return err
	}
	if hasDelete {
		defer os.Remove(yamlFilePath)
	}

	yamlContent, err := os.ReadFile(yamlFilePath)
	if err != nil {
		return err
	}
	dockerYaml, err := docker.NewYaml(yamlContent)
	if err != nil {
		return err
	}
	// 删除compose 前需要先把关联的已有容器网络退出
	for _, item := range dockerYaml.GetExternalLinks() {
		for _, name := range dockerYaml.GetNetworkList() {
			err = docker.Sdk.Client.NetworkDisconnect(docker.Sdk.Ctx, task.Entity.Name+"_"+name, item.ContainerName, true)
			if err != nil {
				return err
			}
		}
	}

	envFile, err := task.getEnvFile()
	if err != nil {
		return err
	}
	defer os.Remove(envFile)

	command := []string{
		"-f", yamlFilePath,
		"-p", task.Entity.Name,
		"--env-file", envFile,
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
	envFile, err := task.getEnvFile()
	if err != nil {
		return err
	}
	defer os.Remove(envFile)

	yamlFilePath, hasDelete, err := task.getYamlFile()
	if err != nil {
		return err
	}
	if hasDelete {
		defer os.Remove(yamlFilePath)
	}

	command := []string{
		"-f", yamlFilePath,
		"-p", task.Entity.Name,
		"--env-file", envFile,
		"--progress", "tty",
		op,
	}
	self.runCommand(command)

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
	if task.Entity.Name == "" {
		return result
	}

	yamlFilePath, hasDelete, err := task.getYamlFile()
	if err != nil {
		return result
	}
	if hasDelete {
		defer os.Remove(yamlFilePath)
	}

	command := []string{
		"-f", yamlFilePath,
		"-p", task.Entity.Name,
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

func (self Compose) Sync() error {
	composeList, _ := dao.Compose.Find()

	composeFileName := []string{
		"docker-compose.yml", "docker-compose.yaml",
		"compose.yml", "compose.yaml",
	}
	rootDir := storage.Local{}.GetComposePath()
	err := filepath.Walk(rootDir, func(path string, info fs.FileInfo, err error) error {
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
								Uri:    rel,
							},
						})
					}
				}
				break
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}
