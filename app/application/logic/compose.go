package logic

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/compose-spec/compose-go/v2/cli"
	"github.com/compose-spec/compose-go/v2/types"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/compose"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/exec"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
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
	ComposeTypeOutPath     = "outPath"
	ComposeStatusWaiting   = "waiting"
	ComposeProjectName     = "dpanel-c-%d"
)

type Compose struct {
}

type composeItem struct {
	Name           string `json:"name"`
	Status         string `json:"status"`
	ConfigFiles    string `json:"configFiles"`
	ConfigFileList []string
}

func (self Compose) Ls() []*composeItem {
	command := []string{
		"ls",
		"--format", "json",
		"--all",
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
	for i, item := range result {
		if strings.Contains(item.ConfigFiles, ",") {
			result[i].ConfigFileList = strings.Split(item.ConfigFiles, ",")
		} else {
			result[i].ConfigFileList = []string{
				item.ConfigFiles,
			}
		}
	}
	return result
}

func (self Compose) Kill() error {
	return exec.Command{}.Kill()
}

func (self Compose) Sync() error {
	composeList, _ := dao.Compose.Find()
	oldComposeName := make([]string, 0)
	for _, item := range composeList {
		if item.Setting.Type == ComposeTypeStoragePath {
			oldComposeName = append(oldComposeName, item.Name)
		}
	}

	composeFileName := []string{
		"docker-compose.yml", "docker-compose.yaml",
		"compose.yml", "compose.yaml",
	}

	pathFindComposeName := make([]string, 0)
	rootDir := storage.Local{}.GetComposePath()
	err := filepath.Walk(rootDir, func(path string, info fs.FileInfo, err error) error {
		for _, suffix := range composeFileName {
			if strings.HasSuffix(path, suffix) {
				rel, _ := filepath.Rel(rootDir, path)
				// 只同步二级目录下的 yaml
				if segments := strings.Split(filepath.Clean(rel), string(filepath.Separator)); len(segments) == 2 {
					name := filepath.Dir(rel)
					pathFindComposeName = append(pathFindComposeName, name)

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
								Status: ComposeStatusWaiting,
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

	deleteList := make([]string, 0)
	for _, name := range oldComposeName {
		if !function.InArray(pathFindComposeName, name) {
			deleteList = append(deleteList, name)
		}
	}
	if !function.IsEmptyArray(deleteList) {
		_, _ = dao.Compose.Where(dao.Compose.Name.In(deleteList...)).Delete()
	}
	if err != nil {
		return err
	}
	return nil
}

func (self Compose) GetTasker(entity *entity.Compose) (*compose.Task, error) {
	yamlFilePath := ""
	if entity.Setting.Type == ComposeTypeServerPath {
		yamlFilePath = entity.Setting.Uri
	} else if entity.Setting.Type == ComposeTypeStoragePath {
		yamlFilePath = filepath.Join(storage.Local{}.GetComposePath(), entity.Setting.Uri)
	} else {
		yamlFilePath = filepath.Join(storage.Local{}.GetComposePath(), entity.Name, "compose.yaml")
		err := os.MkdirAll(filepath.Dir(yamlFilePath), os.ModePerm)
		if err != nil {
			return nil, err
		}
		if entity.Setting.Type == ComposeTypeRemoteUrl {
			response, err := http.Get(entity.Yaml)
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
			entity.Yaml = string(content)
		}
		content := []byte(entity.Yaml)
		if !strings.Contains(entity.Yaml, "!!!dpanel") && entity.Setting.Type == ComposeTypeRemoteUrl {
			content = append([]byte("# !!!dpanel 此文件由 dpanel 面板生成，请勿修改！ \n"), content...)
		}
		err = os.WriteFile(yamlFilePath, content, 0666)
		if err != nil {
			return nil, err
		}
	}
	envWithEquals := entity.Setting.EnvironmentToMappingWithEquals()

	options := []cli.ProjectOptionsFn{
		compose.WithYamlPath(yamlFilePath), cli.WithEnv(envWithEquals),
	}
	// 尝试去工作目录下查询自定义的 override.yaml override.yml 文件进行附加
	overrideFilePath := []string{
		"override.yaml", "override.yml",
	}
	fileList, err := filepath.Glob(filepath.Join(filepath.Join(storage.Local{}.GetComposePath(), entity.Name), "*"))
	if err == nil {
		for _, path := range fileList {
			for _, overrideName := range overrideFilePath {
				if strings.Contains(path, overrideName) {
					options = append(options, compose.WithYamlPath(path))
					continue
				}
			}
		}
	}
	// 最终Yaml需要用到原始的compose，创建一个原始的对象
	originalComposer, err := compose.NewCompose(options...)
	if err != nil {
		return nil, err
	}

	projectName := fmt.Sprintf(ComposeProjectName, entity.ID)
	options = make([]cli.ProjectOptionsFn, 0)
	if entity.ID > 0 {
		// compose 项止名称不允许有大小写，但是compose的目录名可以包含特殊字符，这里统一用id进行区分
		options = append(options, cli.WithName(projectName))
	}
	options = append(options, cli.WithEnv(envWithEquals))
	extProject := compose.Ext{}
	options = append(options, cli.WithExtension(compose.ExtensionName, &extProject))
	// 生成覆盖配置时，需要获取原始yaml的数据，所以这里生构建出原始的compose对象，再进行覆盖。
	// 生成覆盖Yaml
	yamlOverrideFilePath := filepath.Join(storage.Local{}.GetComposePath(), entity.Name, "dpanel-deploy.yaml")
	err = os.MkdirAll(filepath.Dir(yamlOverrideFilePath), os.ModePerm)
	if err != nil {
		return nil, err
	}
	options = append(options, compose.WithYamlPath(yamlFilePath))
	options = append(options, compose.WithYamlPath(yamlOverrideFilePath))

	overrideYaml, err := originalComposer.GetOverrideYaml(entity.Setting.Override)
	if err != nil {
		return nil, err
	}
	err = os.WriteFile(yamlOverrideFilePath, overrideYaml, 0666)
	if err != nil {
		return nil, err
	}

	// 如果面板的 /dpanel 挂载到了宿主机，则重新设置 workDir
	dpanelContainerInfo, _ := docker.Sdk.ContainerInfo(facade.GetConfig().GetString("app.name"))
	for _, mount := range dpanelContainerInfo.Mounts {
		if mount.Type == types.VolumeTypeBind && mount.Destination == "/dpanel" {
			options = append(options, cli.WithWorkingDirectory(filepath.Join(mount.Source, "compose", entity.Name)))
		}
	}
	composer, err := compose.NewCompose(options...)
	if err != nil {
		return nil, err
	}
	overrideYaml, err = composer.Project.MarshalYAML()
	if err != nil {
		return nil, err
	}
	if !bytes.Contains(overrideYaml, []byte("!!!dpanel")) {
		overrideYaml = append([]byte(`# !!!dpanel
# 此文件由 dpanel 面板自动生成，为最终的部署文件，请勿手动修改！
# 如果有修改需求，请操作原始 yaml 文件，或是新建 override.yaml 覆盖文件
`), overrideYaml...)
	}
	err = os.WriteFile(yamlOverrideFilePath, overrideYaml, 0666)
	if err != nil {
		return nil, err
	}
	tasker := compose.NewTasker(projectName, composer)
	return tasker, nil
}
