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
	"strconv"
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

var overrideFileNameSuffix = []string{
	"override.yaml", "override.yml",
}

var composeFileNameSuffix = []string{
	"docker-compose.yml", "docker-compose.yaml",
	"compose.yml", "compose.yaml",
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
	composeList, _ := dao.Compose.Find()
	findComposeList := make(map[string]*entity.Compose)

	for _, item := range self.Ls() {
		if strings.Contains(item.Name, "dpanel-c-") {
			// 找到对应的任务，合并状态数据
			id, err := strconv.Atoi(item.Name[strings.LastIndex(item.Name, "-")+1 : len(item.Name)])
			if err == nil {
				exists, pos := function.FindArrayValueIndex(composeList, "ID", int32(id))
				if exists {
					composeList[pos[0]].Setting.Status = item.Status
				}
			}
		} else {
			findComposeList[item.Name] = &entity.Compose{
				Name:  item.Name,
				Title: "",
				Setting: &accessor.ComposeSettingOption{
					Status: item.Status,
					Uri:    item.ConfigFileList,
					Type:   ComposeTypeOutPath,
				},
			}
		}
	}

	rootDir := storage.Local{}.GetComposePath()
	err := filepath.Walk(rootDir, func(path string, info fs.FileInfo, err error) error {
		for _, suffix := range composeFileNameSuffix {
			if strings.HasSuffix(path, suffix) {
				rel, _ := filepath.Rel(rootDir, path)
				// 只同步二级目录下的 yaml
				if segments := strings.Split(filepath.Clean(rel), string(filepath.Separator)); len(segments) == 2 {
					name := filepath.Dir(rel)
					if findRow, ok := findComposeList[name]; ok {
						findComposeList[name].Title = findRow.Title
						findComposeList[name].Setting.Status = findRow.Setting.Status
						findComposeList[name].Setting.Uri = []string{
							rel,
						}
					} else {
						findComposeList[name] = &entity.Compose{
							Title: "",
							Name:  name,
							Setting: &accessor.ComposeSettingOption{
								Type:   ComposeTypeStoragePath,
								Status: ComposeStatusWaiting,
								Uri: []string{
									rel,
								},
							},
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

	// 循环任务，添加，清理任务
	for _, dbComposeRow := range composeList {
		has := false
		for findKey, findComposeRow := range findComposeList {
			if dbComposeRow.Name == findComposeRow.Name {
				has = true
				dbComposeRow.Setting.Uri = findComposeRow.Setting.Uri
				_, _ = dao.Compose.Where(dao.Compose.ID.Eq(dbComposeRow.ID)).Updates(dbComposeRow)
				delete(findComposeList, findKey)
			}
		}
		if !has && !function.InArray([]string{
			ComposeTypeText, ComposeTypeRemoteUrl,
		}, dbComposeRow.Setting.Type) {
			// 目录已经删除，但是任务还运行，则保留数据
			if dbComposeRow.Setting.Status == ComposeStatusWaiting {
				_, _ = dao.Compose.Where(dao.Compose.ID.Eq(dbComposeRow.ID)).Delete()
			}
		}
	}

	if !function.IsEmptyMap(findComposeList) {
		for _, item := range findComposeList {
			_ = dao.Compose.Create(item)
		}
	}

	if err != nil {
		return err
	}
	return nil
}

func (self Compose) GetTasker(entity *entity.Compose) (*compose.Task, error) {
	workingDir := ""
	// 如果面板的 /dpanel 挂载到了宿主机，则重新设置 workDir
	dpanelContainerInfo, _ := docker.Sdk.ContainerInfo(facade.GetConfig().GetString("app.name"))
	for _, mount := range dpanelContainerInfo.Mounts {
		if mount.Type == types.VolumeTypeBind && mount.Destination == "/dpanel" {
			workingDir = filepath.Join(mount.Source, "compose", entity.Name)
		}
	}

	yamlFilePath := make([]string, 0)
	if entity.Setting.Type == ComposeTypeServerPath {
		yamlFilePath = entity.Setting.Uri
	} else if entity.Setting.Type == ComposeTypeStoragePath {
		for _, item := range entity.Setting.Uri {
			yamlFilePath = append(yamlFilePath, filepath.Join(storage.Local{}.GetComposePath(), item))
		}
	} else if entity.Setting.Type == ComposeTypeOutPath {
		// 外部路径分两种，一种是原目录挂载，二是将Yaml文件放置到存储目录中
		for _, item := range entity.Setting.Uri {
			if filepath.IsAbs(item) {
				yamlFilePath = append(yamlFilePath, item)
			} else {
				yamlFilePath = append(yamlFilePath, filepath.Join(storage.Local{}.GetComposePath(), item))
			}
		}
	} else {
		tempYamlFilePath := filepath.Join(storage.Local{}.GetComposePath(), entity.Name, "compose.yaml")
		err := os.MkdirAll(filepath.Dir(tempYamlFilePath), os.ModePerm)
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
		err = os.WriteFile(tempYamlFilePath, content, 0666)
		if err != nil {
			return nil, err
		}
		yamlFilePath = append(yamlFilePath, tempYamlFilePath)
	}
	envWithEquals := entity.Setting.EnvironmentToMappingWithEquals()
	options := []cli.ProjectOptionsFn{
		cli.WithEnv(envWithEquals),
	}
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
	yamlDeployFileName := filepath.Join(storage.Local{}.GetComposePath(), entity.Name, "dpanel-deploy.yaml")
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

	projectName := fmt.Sprintf(ComposeProjectName, entity.ID)
	options = make([]cli.ProjectOptionsFn, 0)
	if entity.Setting.Type == ComposeTypeOutPath {
		// compose 项止名称不允许有大小写，但是compose的目录名可以包含特殊字符，这里统一用id进行区分
		// 如果是外部任务，则保持原有名称
		projectName = entity.Name
	}
	options = append(options, cli.WithName(projectName))
	options = append(options, cli.WithEnv(envWithEquals))
	options = append(options, compose.WithYamlPath(yamlDeployFileName))

	if workingDir != "" {
		options = append(options, cli.WithWorkingDirectory(workingDir))
	}

	extProject := compose.Ext{}
	options = append(options, cli.WithExtension(compose.ExtensionName, &extProject))

	// 根据数据库中的覆盖配置生成覆盖 yaml
	overrideFileName := filepath.Join(storage.Local{}.GetComposePath(), entity.Name, "dpanel-override.yaml")
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
