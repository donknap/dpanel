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
	exec2 "os/exec"
	"path/filepath"
	"strings"
)

const (
	ComposeProjectName           = "dpanel-c-%s"
	ComposeProjectDeployFileName = "dpanel-deploy.yaml"
	ComposeProjectEnvFileName    = ".dpanel.env"
)

var composeFileNameSuffix = []string{
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
}

func (self Compose) Ls() []*composeItem {
	command := []string{
		"ls",
		"--format", "json",
		"--all",
	}

	out := ""
	if _, err := exec2.LookPath("docker-compose"); err == nil {
		out = exec.Command{}.RunWithResult(&exec.RunCommandOption{
			CmdName: "docker-compose",
			CmdArgs: command,
			Env:     docker.Sdk.Env,
		})
	} else {
		out = exec.Command{}.RunWithResult(&exec.RunCommandOption{
			CmdName: "docker",
			CmdArgs: append(append(docker.Sdk.ExtraParams, "compose"), command...),
		})
	}

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
	// 重置所有任务状态为等待
	composeList, _ := dao.Compose.Find()
	for i, _ := range composeList {
		composeList[i].Setting.Status = accessor.ComposeStatusWaiting
	}
	// 查找部署过的compose任务，如果是 dpanel-c- 开头表示是系统部署的任务，需要重新定义一下 name
	// 运行中的任务如果是面板部署的，将数据库中的数据替换到查找到的运行任务
	// 非面板部署的任务记录下 Yaml 所在位置，在管理页面中确认是否可以找到文件进行管理
	findComposeList := make(map[string]*entity.Compose)
	for _, item := range self.Ls() {
		findRow := &entity.Compose{
			Name:  item.Name,
			Title: "",
			Setting: &accessor.ComposeSettingOption{
				Status: item.Status,
				Uri:    item.ConfigFileList,
				Type:   accessor.ComposeTypeOutPath,
			},
		}

		has := false
		if strings.HasPrefix(item.Name, "dpanel-c-") {
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

	// 此时 findComposeList 中仅包含的是运行中的任务
	// 遍历存储目录查找所有的任务文件
	// 查找到任务文件如果在数据库中标记是文本或是远程地址，则直接跳过，此目录为系统生成的临时部署目录，用户修改无效。
	// 目录中的任务已经在运行中，仅需要将 uri 重新赋值即可，状态这些不需要再重新赋值
	// 目录中的任务没有运行，则还需要再去数据库中查找一下，需要将数据库中的数据同步到查找列表中
	// 目录中的任务数据库中也没有，则添加需要创建
	err := filepath.Walk(rootDir, func(path string, info fs.FileInfo, err error) error {
		for _, suffix := range composeFileNameSuffix {
			if strings.HasSuffix(path, suffix) {
				rel, _ := filepath.Rel(rootDir, path)
				// 只同步二级目录下的 yaml
				if segments := strings.Split(filepath.Clean(rel), string(filepath.Separator)); len(segments) == 2 {
					// 强制转为小写
					name := strings.ToLower(filepath.Dir(rel))

					if _, ok := findComposeList[name]; ok {
						if function.InArray([]string{
							accessor.ComposeTypeText, accessor.ComposeTypeRemoteUrl, accessor.ComposeTypeStore,
						}, findComposeList[name].Setting.Type) {
							break
						}
						findComposeList[name].Setting.Uri[0] = rel
					} else {
						exists, pos := function.FindArrayValueIndex(composeList, "Name", name)
						if exists {
							dbComposeRow := composeList[pos[0]]
							// 文本和远程地址是主动添加，无论如何都要保留记录
							if function.InArray([]string{
								accessor.ComposeTypeText, accessor.ComposeTypeRemoteUrl,
							}, dbComposeRow.Setting.Type) {
								break
							}
							if dbComposeRow.Setting.Type == accessor.ComposeTypeStore {

							} else {
								dbComposeRow.Setting.Type = accessor.ComposeTypeStoragePath
								dbComposeRow.Setting.Uri[0] = rel
							}
							findComposeList[dbComposeRow.Name] = dbComposeRow
						} else {
							findRow := &entity.Compose{
								Name:  name,
								Title: "",
								Setting: &accessor.ComposeSettingOption{
									Type:   accessor.ComposeTypeStoragePath,
									Status: accessor.ComposeStatusWaiting,
									Uri: []string{
										rel,
									},
								},
							}
							findComposeList[name] = findRow
						}
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
		if !has {
			if function.InArray([]string{
				accessor.ComposeTypeOutPath, accessor.ComposeTypeStoragePath, accessor.ComposeTypeStore,
			}, dbComposeRow.Setting.Type) {
				_, _ = dao.Compose.Where(dao.Compose.ID.Eq(dbComposeRow.ID)).Delete()
			}

			if function.InArray([]string{
				accessor.ComposeTypeText, accessor.ComposeTypeRemoteUrl,
			}, dbComposeRow.Setting.Type) {
				_, _ = dao.Compose.Where(dao.Compose.ID.Eq(dbComposeRow.ID)).Updates(&entity.Compose{
					Setting: dbComposeRow.Setting,
				})
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
	workingDir := storage.Local{}.GetComposePath()

	// 如果面板的 /dpanel 挂载到了宿主机，则重新设置 workDir
	dpanelContainerInfo, _ := docker.Sdk.ContainerInfo(facade.GetConfig().GetString("app.name"))
	for _, mount := range dpanelContainerInfo.Mounts {
		if mount.Type == types.VolumeTypeBind && mount.Destination == "/dpanel" {
			// 当容器挂载了外部目录，创建时必须保证此目录有文件可以访问。否则相对目录会错误
			if _, err := os.Stat(mount.Source); err != nil {
				_ = os.MkdirAll(mount.Source, os.ModePerm)
				err = os.Symlink(storage.Local{}.GetComposePath(), filepath.Join(mount.Source, "compose"))
				if err != nil {
					return nil, err
				}
			}
			workingDir = filepath.Join(mount.Source, "compose")
		}
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
