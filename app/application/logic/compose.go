package logic

import (
	"encoding/json"
	"github.com/compose-spec/compose-go/v2/cli"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/compose"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/exec"
	"github.com/donknap/dpanel/common/service/storage"
	"io"
	"io/fs"
	"net/http"
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

func (self ComposeTaskOption) Compose() (*compose.Wrapper, error) {
	options := make([]cli.ProjectOptionsFn, 0)
	if function.IsEmptyArray(self.Entity.Setting.Environment) {
		options = append(options, cli.WithEnv(self.Entity.Setting.GetEnvList()))
	}
	options = append(options, cli.WithName(self.Entity.Name))

	// todo: 还需要添加覆盖配置

	if self.Entity.Setting.Type == ComposeTypeServerPath {
		options = append(options, compose.WithYamlPath(self.Entity.Yaml))
	}
	if self.Entity.Setting.Type == ComposeTypeStoragePath {
		options = append(options, compose.WithYamlPath(filepath.Join(storage.Local{}.GetComposePath(), self.Entity.Yaml)))
	}
	if self.Entity.Setting.Type == ComposeTypeText || self.Entity.Setting.Type == "" {
		options = append(options, compose.WithYamlString(filepath.Join(self.Entity.Name, "compose.yaml"), []byte(self.Entity.Yaml)))
	}
	if self.Entity.Setting.Type == ComposeTypeRemoteUrl {
		response, err := http.Get(self.Entity.Yaml)
		if err != nil {
			return nil, err
		}
		defer func() {
			_ = response.Body.Close()
		}()
		content, err := io.ReadAll(response.Body)
		if err != nil {
			return nil, err
		}
		options = append(options, compose.WithYamlString(filepath.Join(self.Entity.Name, "compose.yaml"), content))
	}
	composer, err := compose.NewCompose(options...)
	if err != nil {
		return nil, nil
	}
	return composer, nil
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
	composer, err := task.Compose()
	if err != nil {
		return err
	}
	cmd := composer.GetBaseCommand()
	self.runCommand(append(cmd, "--progress", "tty", "up", "-d"))

	// todo 部署完成后还需要把外部容器加入到对应的网络中
	// 如果 compose 中未指定网络，则默认的名称为 项目名_default

	return nil
}

func (self Compose) Destroy(task *ComposeTaskOption) error {
	composer, err := task.Compose()
	if err != nil {
		return err
	}
	cmd := composer.GetBaseCommand()
	// todo 删除compose 前需要先把关联的已有容器网络退出

	cmd = append(cmd, "--progress", "tty", "down")
	if task.DeleteImage {
		cmd = append(cmd, "--rmi", "all")
	}
	self.runCommand(cmd)
	return nil
}

func (self Compose) Ctrl(task *ComposeTaskOption, op string) error {
	composer, err := task.Compose()
	if err != nil {
		return err
	}
	cmd := composer.GetBaseCommand()
	self.runCommand(append(cmd, "--progress", "tty", op))

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

	composer, err := task.Compose()
	if err != nil {
		return result
	}
	cmd := composer.GetBaseCommand()
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
