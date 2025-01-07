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
	ComposeProjectName           = "dpanel-c-%s"
	ComposeProjectDeployFileName = "dpanel-deploy.yaml"
	ComposeProjectEnvFileName    = ".dpanel.env"
)

var ComposeFileNameSuffix = []string{
	"docker-compose.yml", "docker-compose.yaml",
	"compose.yml", "compose.yaml",
}

type StoreItem struct {
	Title       string `json:"title"`
	Name        string `json:"name"`
	Logo        string `json:"logo"`
	Description string `json:"description"`
}

type Compose struct {
}

type composeItem struct {
	Name           string `json:"name"`
	Status         string `json:"status"`
	ConfigFiles    string `json:"configFiles"`
	ConfigFileList []string
	IsDPanel       bool
}

func (self Compose) Ls() []*composeItem {
	command := []string{
		"ls",
		"--format", "json",
		"--all",
	}
	result := make([]*composeItem, 0)
	cmd, err := exec.New(docker.Sdk.GetComposeCmd(command...)...)
	if err != nil {
		return result
	}
	err = json.Unmarshal([]byte(cmd.RunWithResult()), &result)
	if err != nil {
		return result
	}
	for i, item := range result {
		if strings.HasPrefix(item.Name, "dpanel-c") {
			item.IsDPanel = true
		}
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

func (self Compose) LsItem(name string) *composeItem {
	ls := self.Ls()
	for _, item := range ls {
		if item.Name == name || item.Name == fmt.Sprintf(ComposeProjectName, name) {
			return item
		}
	}
	return &composeItem{
		Name:           name,
		Status:         accessor.ComposeStatusWaiting,
		ConfigFiles:    "",
		ConfigFileList: make([]string, 0),
		IsDPanel:       true,
	}
}

func (self Compose) Kill() error {
	return exec.Command{}.Kill()
}

func (self Compose) FindRunTask() map[string]*entity.Compose {
	findComposeList := make(map[string]*entity.Compose)
	for _, item := range self.Ls() {
		if strings.HasPrefix(item.Name, "dpanel-c-") {
			item.Name = item.Name[9:]
		}
		findRow := &entity.Compose{
			Name:  item.Name,
			Title: "",
			Setting: &accessor.ComposeSettingOption{
				Status:        item.Status,
				Uri:           item.ConfigFileList,
				Type:          accessor.ComposeTypeOutPath,
				DockerEnvName: docker.Sdk.Name,
			},
		}
		findComposeList[item.Name] = findRow
	}
	return findComposeList
}

func (self Compose) FindPathTask(rootDir string) map[string]*entity.Compose {
	// 查询当前运行中的和目录中的 compose 任务
	// 查找运行中的任务，如果是 dpanel-c- 开头表示是系统部署的任务，需要重新定义一下 name
	// 非面板部署的任务记录下 Yaml 所在位置，如果在目录中找到对应的名称则重新定义 uri
	findComposeList := make(map[string]*entity.Compose)
	_ = filepath.Walk(rootDir, func(path string, info fs.FileInfo, err error) error {
		for _, suffix := range ComposeFileNameSuffix {
			if strings.HasSuffix(path, suffix) {
				rel, _ := filepath.Rel(rootDir, path)
				// 只同步二级目录下的 yaml
				if segments := strings.Split(filepath.Clean(rel), string(filepath.Separator)); len(segments) == 2 {
					// 强制转为小写
					name := strings.ToLower(filepath.Dir(rel))
					findRow := &entity.Compose{
						Name:  name,
						Title: "",
						Setting: &accessor.ComposeSettingOption{
							Type:   accessor.ComposeTypeStoragePath,
							Status: "",
							Uri: []string{
								rel,
							},
						},
					}
					findComposeList[name] = findRow
				}
				break
			}
		}
		return nil
	})
	return findComposeList
}

// 同步当前挂载目录中的 compose
func (self Compose) Sync(dockerEnvName string) error {
	var rootDir string
	if dockerEnvName == docker.DefaultClientName {
		rootDir = storage.Local{}.GetComposePath()
	} else {
		rootDir = filepath.Join(filepath.Dir(storage.Local{}.GetComposePath()), "compose-"+dockerEnvName)
	}
	findComposeList := self.FindPathTask(rootDir)
	for i, _ := range findComposeList {
		findComposeList[i].Setting.DockerEnvName = dockerEnvName
	}

	// 重置所有任务状态为等待
	composeList, _ := dao.Compose.Find()

	// 循环任务，添加，清理任务
	for _, dbComposeRow := range composeList {
		if find, ok := findComposeList[dbComposeRow.Name]; ok && find.Setting.DockerEnvName == dbComposeRow.Setting.DockerEnvName {
			delete(findComposeList, dbComposeRow.Name)
		} else {
			// 除非任务的类型是属于当前的环境才执行删除
			if function.InArray([]string{
				accessor.ComposeTypeOutPath, accessor.ComposeTypeStoragePath, accessor.ComposeTypeStore,
			}, dbComposeRow.Setting.Type) && dbComposeRow.Setting.DockerEnvName == docker.Sdk.Name {
				_, _ = dao.Compose.Where(dao.Compose.ID.Eq(dbComposeRow.ID)).Delete()
			}
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
	workingDir := entity.Setting.GetWorkingDir()

	// 如果面板的 /dpanel 挂载到了宿主机，则重新设置 workDir
	dpanelContainerInfo, _ := docker.Sdk.ContainerInfo(facade.GetConfig().GetString("app.name"))
	for _, mount := range dpanelContainerInfo.Mounts {
		if mount.Type == types.VolumeTypeBind && mount.Destination == "/dpanel" {
			// 当容器挂载了外部目录，创建时必须保证此目录有文件可以访问。否则相对目录会错误
			if _, err := os.Stat(mount.Source); err != nil {
				_ = os.MkdirAll(mount.Source, os.ModePerm)
				err = os.Symlink(workingDir, filepath.Join(mount.Source, filepath.Base(workingDir)))
				if err != nil {
					return nil, err
				}
			}
			workingDir = filepath.Join(mount.Source, filepath.Base(workingDir))
		}
	}

	// 判断当前任务是否是外部任务
	composeRun := self.LsItem(entity.Name)
	if !composeRun.IsDPanel {
		entity.Setting.Type = accessor.ComposeTypeOutPath
	}

	// 如果是远程文件，每次都获取最新的 yaml 文件进行覆盖
	if entity.Setting.Type == accessor.ComposeTypeRemoteUrl {
		tempYamlFilePath := entity.Setting.GetUriFilePath()
		err := os.MkdirAll(filepath.Dir(tempYamlFilePath), os.ModePerm)
		if err != nil {
			return nil, err
		}
		response, err := http.Get(entity.Setting.RemoteUrl)
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
		err = os.WriteFile(tempYamlFilePath, self.makeDeployYamlHeader(content), 0666)
		if err != nil {
			return nil, err
		}
	}

	taskFileDir := filepath.Join(workingDir, filepath.Dir(entity.Setting.Uri[0]))

	yamlFilePath := make([]string, 0)

	if entity.Setting.Type == accessor.ComposeTypeOutPath {
		// 外部路径分两种，一种是原目录挂载，二是将Yaml文件放置到存储目录中
		for _, item := range entity.Setting.Uri {
			if filepath.IsAbs(item) {
				yamlFilePath = append(yamlFilePath, item)
			} else {
				yamlFilePath = append(yamlFilePath, filepath.Join(taskFileDir, filepath.Base(item)))
			}
		}
	} else {
		for _, item := range entity.Setting.Uri {
			yamlFilePath = append(yamlFilePath, filepath.Join(taskFileDir, filepath.Base(item)))
		}
	}

	options := make([]cli.ProjectOptionsFn, 0)
	for _, path := range yamlFilePath {
		options = append(options, compose.WithYamlPath(path))
	}

	if !function.IsEmptyArray(entity.Setting.Environment) {
		globalEnv := make([]string, 0)
		for _, item := range entity.Setting.Environment {
			globalEnv = append(globalEnv, fmt.Sprintf("%s=%s", item.Name, compose.ReplacePlaceholder(item.Value)))
		}
		envFileName := filepath.Join(taskFileDir, ComposeProjectEnvFileName)
		err := os.MkdirAll(filepath.Dir(envFileName), os.ModePerm)
		if err != nil {
			return nil, err
		}
		err = os.WriteFile(envFileName, []byte(strings.Join(globalEnv, "\n")), 0666)
		options = append(options, cli.WithEnvFiles(envFileName))
		options = append(options, cli.WithEnv(globalEnv))
	}

	projectName := fmt.Sprintf(ComposeProjectName, entity.Name)
	if entity.Setting.Type == accessor.ComposeTypeOutPath {
		// compose 项止名称不允许有大小写，但是compose的目录名可以包含特殊字符，这里统一用id进行区分
		// 如果是外部任务，则保持原有名称
		projectName = entity.Name
	}
	options = append(options, cli.WithName(projectName))

	// 最终Yaml需要用到原始的compose，创建一个原始的对象
	originalComposer, err := compose.NewCompose(options...)
	if err != nil {
		return nil, err
	}

	tasker := &compose.Task{
		Name:     projectName,
		Composer: originalComposer,
		Status:   composeRun.Status,
	}
	return tasker, nil
}

func (self Compose) makeDeployYamlHeader(yaml []byte) []byte {
	if !bytes.Contains(yaml, []byte("!!!dpanel")) {
		yaml = append([]byte(`# !!!dpanel
# 此文件由 dpanel 面板自动生成，请勿手动修改！！！
# 如果有修改需求，请编辑原始 yaml 文件或是 Compose 任务。
`), yaml...)
	}
	return yaml
}
