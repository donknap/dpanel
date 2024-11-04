package logic

import (
	"bytes"
	"encoding/json"
	"errors"
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
	ComposeTypeText              = "text"
	ComposeTypeRemoteUrl         = "remoteUrl"
	ComposeTypeServerPath        = "serverPath"
	ComposeTypeStoragePath       = "storagePath"
	ComposeTypeOutPath           = "outPath"
	ComposeStatusWaiting         = "waiting"
	ComposeProjectName           = "dpanel-c-%s"
	ComposeProjectDeployFileName = "dpanel-deploy.yaml"
)

var overrideFileNameSuffix = []string{
	"override.yaml", "override.yml",
}

var composeFileNameSuffix = []string{
	"docker-compose.yml", "docker-compose.yaml",
	"compose.yml", "compose.yaml",
}

var dockerEnvNameSuffix = []string{
	".yaml", ".yml",
}

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
	out := exec.Command{}.RunWithResult(&exec.RunCommandOption{
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

func (self Compose) LsItem(name string) (*composeItem, error) {
	for _, item := range self.Ls() {
		if item.Name == name {
			return item, nil
		}
	}
	return nil, errors.New("task not running")
}

func (self Compose) Kill() error {
	return exec.Command{}.Kill()
}

// Sync 同步存储目录中的任务及已运行的外部任务，并同步当前任务的状态
func (self Compose) Sync() error {
	rootDir := storage.Local{}.GetComposePath()

	composeList, _ := dao.Compose.Find()
	for i, _ := range composeList {
		composeList[i].Setting.Status = ComposeStatusWaiting
	}

	// 查找部署过的compose任务，如果是 dpanel-c- 开头表示是系统部署的任务，需要重新定义一下 name
	// 否则是外部的任务
	findComposeList := make(map[string]*entity.Compose)
	for _, item := range self.Ls() {
		findRow := &entity.Compose{
			Name:  item.Name,
			Title: "",
			Setting: &accessor.ComposeSettingOption{
				Status: item.Status,
				Uri:    item.ConfigFileList,
				Type:   ComposeTypeOutPath,
			},
		}

		has := false
		if strings.HasPrefix(item.Name, "dpanel-c-") {
			// 找到对应的任务，从数据库中获取数据
			name := item.Name[9:]
			exists, pos := function.FindArrayValueIndex(composeList, "Name", name)
			if exists {
				has = true
				dbComposeRow := composeList[pos[0]]
				dbComposeRow.Setting.Status = item.Status
				findComposeList[dbComposeRow.Name] = dbComposeRow
			}
		}

		if !has {
			findComposeList[item.Name] = findRow
		}
	}

	err := filepath.Walk(rootDir, func(path string, info fs.FileInfo, err error) error {
		for _, suffix := range composeFileNameSuffix {
			if strings.HasSuffix(path, suffix) {
				rel, _ := filepath.Rel(rootDir, path)
				// 只同步二级目录下的 yaml
				if segments := strings.Split(filepath.Clean(rel), string(filepath.Separator)); len(segments) == 2 {
					// 强制转为小写
					name := strings.ToLower(filepath.Dir(rel))
					if _, ok := findComposeList[name]; ok {
						if !function.InArray([]string{
							ComposeTypeText, ComposeTypeRemoteUrl,
						}, findComposeList[name].Setting.Type) {
							findComposeList[name].Setting.Uri = []string{
								rel,
							}
						}
					} else {
						exists, pos := function.FindArrayValueIndex(composeList, "Name", name)
						if exists {
							dbComposeRow := composeList[pos[0]]
							// 如果遇到与数据同名的非存储任务，则忽略
							if !function.InArray([]string{
								ComposeTypeText, ComposeTypeRemoteUrl,
							}, dbComposeRow.Setting.Type) {
								dbComposeRow.Setting.Type = ComposeTypeStoragePath
								findComposeList[dbComposeRow.Name] = dbComposeRow
							}
						} else {
							findRow := &entity.Compose{
								Name:  name,
								Title: "",
								Setting: &accessor.ComposeSettingOption{
									Type:   ComposeTypeStoragePath,
									Status: ComposeStatusWaiting,
									Uri: []string{
										rel,
									},
								},
							}
							findComposeList[name] = findRow
						}
					}

					// 查找当前目录是否包含 override yaml
					for _, overridePath := range self.FindPathOverrideYaml(filepath.Dir(path)) {
						rel, _ = filepath.Rel(rootDir, overridePath)
						findComposeList[name].Setting.Uri = append(findComposeList[name].Setting.Uri, rel)
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

	// 循环任务，添加，清理任务
	for _, dbComposeRow := range composeList {
		has := false
		if findComposeRow, ok := findComposeList[dbComposeRow.Name]; ok {
			has = true
			_, _ = dao.Compose.Where(dao.Compose.ID.Eq(dbComposeRow.ID)).Updates(&entity.Compose{
				Setting: findComposeRow.Setting,
			})
			delete(findComposeList, dbComposeRow.Name)
		}
		//
		if !has && function.InArray([]string{
			ComposeTypeOutPath, ComposeTypeStoragePath,
		}, dbComposeRow.Setting.Type) {
			if dbComposeRow.Setting.Type == ComposeTypeOutPath {
				_ = os.RemoveAll(filepath.Join(storage.Local{}.GetComposePath(), filepath.Dir(dbComposeRow.Setting.Uri[0])))
			}
			_, _ = dao.Compose.Where(dao.Compose.ID.Eq(dbComposeRow.ID)).Delete()
		}
	}

	if !function.IsEmptyMap(findComposeList) {
		for _, item := range findComposeList {
			_ = dao.Compose.Create(item)
		}
	}
	return nil
}

func (self Compose) GetTasker(entity *entity.Compose) (*compose.Task, error) {
	var taskFileDir string
	if entity.Setting.Type == ComposeTypeStoragePath {
		taskFileDir = filepath.Join(storage.Local{}.GetComposePath(), filepath.Dir(entity.Setting.Uri[0]))
	} else {
		taskFileDir = filepath.Join(storage.Local{}.GetComposePath(), entity.Name)
	}
	workingDir := ""
	// 如果面板的 /dpanel 挂载到了宿主机，则重新设置 workDir
	dpanelContainerInfo, _ := docker.Sdk.ContainerInfo(facade.GetConfig().GetString("app.name"))
	for _, mount := range dpanelContainerInfo.Mounts {
		if mount.Type == types.VolumeTypeBind && mount.Destination == "/dpanel" {
			workingDir = filepath.Join(mount.Source, "compose", filepath.Base(taskFileDir))
		}
	}

	yamlFilePath := make([]string, 0)
	if entity.Setting.Type == ComposeTypeServerPath {
		yamlFilePath = entity.Setting.Uri
	} else if entity.Setting.Type == ComposeTypeStoragePath {
		for _, item := range entity.Setting.Uri {
			yamlFilePath = append(yamlFilePath, filepath.Join(taskFileDir, filepath.Base(item)))
		}
	} else if entity.Setting.Type == ComposeTypeOutPath {
		// 外部路径分两种，一种是原目录挂载，二是将Yaml文件放置到存储目录中
		for _, item := range entity.Setting.Uri {
			if filepath.IsAbs(item) {
				yamlFilePath = append(yamlFilePath, item)
			} else {
				yamlFilePath = append(yamlFilePath, filepath.Join(taskFileDir, filepath.Base(item)))
			}
		}
	} else {
		tempYamlFilePath := filepath.Join(taskFileDir, "compose.yaml")
		err := os.MkdirAll(filepath.Dir(tempYamlFilePath), os.ModePerm)
		if err != nil {
			return nil, err
		}
		yaml := []byte(entity.Yaml)
		if entity.Setting.Type == ComposeTypeRemoteUrl {
			response, err := http.Get(entity.Setting.Uri[0])
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
			yaml = content
		}
		if !strings.Contains(entity.Yaml, "!!!dpanel") && entity.Setting.Type == ComposeTypeRemoteUrl {
			yaml = append([]byte("# !!!dpanel 此文件由 dpanel 面板生成，请勿修改！ \n"), yaml...)
		}
		err = os.WriteFile(tempYamlFilePath, yaml, 0666)
		if err != nil {
			return nil, err
		}
		yamlFilePath = append(yamlFilePath, tempYamlFilePath)
	}
	options := []cli.ProjectOptionsFn{}
	for _, path := range yamlFilePath {
		options = append(options, compose.WithYamlPath(path))
	}
	if workingDir != "" {
		options = append(options, cli.WithWorkingDirectory(workingDir))
	}
	// 最终Yaml需要用到原始的compose，创建一个原始的对象
	originalComposer, err := compose.NewCompose(options...)
	if err != nil {
		return nil, err
	}

	// 最终部署 yaml 文件
	// 先用原始 compose 生成该文件，再添加面板数据库中的 override 参数，再生成一次
	yamlDeployFileName := filepath.Join(taskFileDir, ComposeProjectDeployFileName)
	err = os.MkdirAll(filepath.Dir(yamlDeployFileName), os.ModePerm)
	if err != nil {
		return nil, err
	}
	overrideYaml, err := originalComposer.Project.MarshalYAML()
	if err != nil {
		return nil, err
	}
	err = os.WriteFile(yamlDeployFileName, self.makeDeployYamlHeader(overrideYaml), 0666)
	if err != nil {
		return nil, err
	}

	projectName := fmt.Sprintf(ComposeProjectName, entity.Name)
	options = make([]cli.ProjectOptionsFn, 0)
	if entity.Setting.Type == ComposeTypeOutPath {
		// compose 项止名称不允许有大小写，但是compose的目录名可以包含特殊字符，这里统一用id进行区分
		// 如果是外部任务，则保持原有名称
		projectName = entity.Name
	}
	options = append(options, cli.WithName(projectName))
	options = append(options, compose.WithYamlPath(yamlDeployFileName))

	if workingDir != "" {
		options = append(options, cli.WithWorkingDirectory(workingDir))
	}

	extProject := compose.Ext{}
	options = append(options, cli.WithExtension(compose.ExtensionName, &extProject))

	// 根据数据库中的覆盖配置生成覆盖 yaml
	overrideFileName := filepath.Join(taskFileDir, "dpanel-override.yaml")
	overrideYaml, err = originalComposer.GetOverrideYaml(entity.Setting.Override)
	if err != nil {
		return nil, err
	}
	err = os.WriteFile(overrideFileName, overrideYaml, 0666)
	if err != nil {
		return nil, err
	}
	options = append(options, compose.WithYamlPath(overrideFileName))
	defer os.Remove(overrideFileName)

	// 最后再添加附加环境 yaml
	for _, suffix := range dockerEnvNameSuffix {
		path := filepath.Join(taskFileDir, docker.Sdk.Host+suffix)
		_, err = os.Stat(filepath.Join(taskFileDir, docker.Sdk.Host+suffix))
		if err == nil {
			options = append(options, compose.WithYamlPath(path))
			break
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
	err = os.WriteFile(yamlDeployFileName, self.makeDeployYamlHeader(overrideYaml), 0666)
	if err != nil {
		return nil, err
	}
	tasker := &compose.Task{
		Name:     projectName,
		Composer: composer,
		Original: originalComposer,
	}
	return tasker, nil
}

func (self Compose) FindPathOverrideYaml(path string) []string {
	find := make([]string, 0)
	fileList, err := filepath.Glob(filepath.Join(path, "*"))
	if err == nil {
		for _, overridePath := range fileList {
			for _, overrideSuffix := range overrideFileNameSuffix {
				if strings.Contains(overridePath, overrideSuffix) {
					find = append(find, overridePath)
					continue
				}
			}
		}
	}
	return find
}

func (self Compose) makeDeployYamlHeader(yaml []byte) []byte {
	if !bytes.Contains(yaml, []byte("!!!dpanel")) {
		yaml = append([]byte(`# !!!dpanel
# 此文件由 dpanel 面板自动生成，为最终的部署文件，请勿手动修改！
# 如果有修改需求，请操作原始 yaml 文件，或是新建 override.yaml 覆盖文件
`), yaml...)
	}
	return yaml
}
