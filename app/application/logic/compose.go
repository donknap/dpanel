package logic

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/compose-spec/compose-go/v2/cli"
	"github.com/compose-spec/compose-go/v2/types"
	"github.com/docker/docker/api/types/container"
	"github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/compose"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/exec"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/define"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

func (self Compose) Get(key string) (*entity.Compose, error) {
	runTaskList := self.FindRunTask()

	if id, err := strconv.Atoi(key); err == nil {
		if row, err := dao.Compose.Where(dao.Compose.ID.Eq(int32(id))).First(); err == nil {
			if run, ok := runTaskList[row.Name]; ok {
				row.Setting.Status = run.Setting.Status
			} else {
				row.Setting.Status = accessor.ComposeStatusWaiting
			}
			return row, nil
		}
		return nil, errors.New("db compose not found")
	} else {
		// 尝试通过标识查询一次
		if row, err := dao.Compose.Where(dao.Compose.Name.Eq(key)).First(); err == nil {
			if run, ok := runTaskList[row.Name]; ok {
				row.Setting.Status = run.Setting.Status
			} else {
				row.Setting.Status = accessor.ComposeStatusWaiting
			}
			return row, nil
		}
		// 尝试去掉前缀先查询一次
		if item, ok := runTaskList[strings.ReplaceAll(key, define.ComposeProjectPrefix, "")]; ok {
			return item, nil
		}
		if item, ok := runTaskList[key]; ok {
			return item, nil
		}
		// 尝试再查询一下子任务
		if name, task, ok := strings.Cut(key, "@"); ok {
			if composeRow, err := self.Get(name); err == nil {
				taskList := self.FindPathTask(filepath.Dir(composeRow.Setting.GetUriFilePath()))
				if v, ok := taskList[task]; ok {
					// 目录需要添加上父级目录
					v.Name = fmt.Sprintf("%s@%s", name, v.Name)
					v.Setting.Uri = function.PluckArrayWalk(v.Setting.Uri, func(item string) (string, bool) {
						return filepath.Join(name, item), true
					})
					return v, nil
				}
			}
		}
		return nil, errors.New("run compose not found")
	}
}

type ComposeLsItem struct {
	Name           string `json:"name"`
	Status         string `json:"status"`
	ConfigFiles    string `json:"configFiles"`
	ConfigFileList []string
	IsDPanel       bool
}

func (self Compose) Ls() []*ComposeLsItem {
	command := []string{
		"ls",
		"--format", "json",
		"--all",
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
	}()

	result := make([]*ComposeLsItem, 0)
	options := docker.Sdk.GetComposeCmd(command...)
	options = append(options, exec.WithCtx(ctx))
	cmd, err := exec.New(options...)
	if err != nil {
		slog.Debug("compose ls", "error", err)
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

func (self Compose) LsItem(name string) *ComposeLsItem {
	ls := self.Ls()
	for _, item := range ls {
		if item.Name == name || item.Name == fmt.Sprintf(define.ComposeProjectName, name) {
			return item
		}
	}
	return &ComposeLsItem{
		Name:           name,
		Status:         accessor.ComposeStatusWaiting,
		ConfigFiles:    "",
		ConfigFileList: make([]string, 0),
		IsDPanel:       true,
	}
}

func (self Compose) FindRunTask() map[string]*entity.Compose {
	dpanelMountDir := ""

	dpanelContainerInfo, _ := logic.Setting{}.GetDPanelInfo()
	for _, mount := range dpanelContainerInfo.Mounts {
		if mount.Type == types.VolumeTypeBind && mount.Destination == "/dpanel" {
			dpanelMountDir = mount.Source
		}
	}

	findComposeList := make(map[string]*entity.Compose)

	for _, item := range self.Ls() {
		if strings.HasPrefix(item.Name, define.ComposeProjectPrefix) {
			item.Name = item.Name[9:]
		}
		// 如果外部任务文件可以访问，则正常管理
		outComposeFileExists := true
		for _, file := range item.ConfigFileList {
			if _, err := os.Stat(file); err != nil {
				// 如果外部任务是 dpanel-c 开头的，还需要将文件路径变更为容器内的实际目录，再尝试查找一次
				if item.IsDPanel && dpanelMountDir != "" {
					rel, _ := filepath.Rel(dpanelMountDir, file)
					if _, err := os.Stat(filepath.Join("/dpanel", rel)); err != nil {
						outComposeFileExists = false
					}
				} else {
					outComposeFileExists = false
				}
			}
		}

		findRow := &entity.Compose{
			Name:  item.Name,
			Title: "",
			Setting: &accessor.ComposeSettingOption{
				Status:        item.Status,
				Uri:           item.ConfigFileList,
				Type:          accessor.ComposeTypeOutPath,
				DockerEnvName: docker.Sdk.Name,
				Environment:   make([]docker.EnvItem, 0),
			},
		}
		if !outComposeFileExists {
			findRow.Setting.Type = accessor.ComposeTypeDangling
		}

		findComposeList[item.Name] = findRow
	}
	return findComposeList
}

func (self Compose) FindPathTask(rootDir string) map[string]*entity.Compose {
	// 查询当前运行中的和目录中的 compose 任务
	// 查找运行中的任务，如果是 dpanel-c- 开头表示是系统部署的任务，需要重新定义一下 name
	// 非面板部署的任务记录下 Yaml 所在位置，如果在目录中找到对应的名称则重新定义 uri
	if _, err := os.Stat(rootDir); err != nil {
		slog.Error("compose sync path not found", "error", err)
		return make(map[string]*entity.Compose)
	}

	var linkRealPath string
	// 如果是软链接，获取到实际指向的目录
	if fileInfo, err := os.Lstat(rootDir); err == nil && fileInfo.Mode()&os.ModeSymlink == os.ModeSymlink {
		if linkRealPath, err = os.Readlink(rootDir); err == nil {
			rootDir = linkRealPath
		}
	}

	slog.Debug("compose find yaml in path", "path", rootDir, "link path", linkRealPath)

	findComposeList := make(map[string]*entity.Compose)
	pathList, err := os.ReadDir(rootDir)
	if err != nil {
		return findComposeList
	}

	for _, path := range pathList {
		if !path.IsDir() {
			continue
		}
		for _, suffix := range ComposeFileNameSuffix {
			relYamlFilePath := filepath.Join(path.Name(), suffix)
			name := strings.ToLower(path.Name())
			if _, err = os.Stat(filepath.Join(rootDir, relYamlFilePath)); err == nil {
				// 强制转为小写
				findRow := &entity.Compose{
					Name:  name,
					Title: "",
					Setting: &accessor.ComposeSettingOption{
						Type:   accessor.ComposeTypeStoragePath,
						Status: "",
						Uri: []string{
							relYamlFilePath,
						},
					},
				}
				findComposeList[name] = findRow
				break
			}
		}
	}
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
	// windows 下无法使用 link 目录对齐到宿主机目录
	linkComposePath := ""
	dpanelContainerInfo, _ := logic.Setting{}.GetDPanelInfo()
	for _, mount := range dpanelContainerInfo.Mounts {
		if mount.Type == types.VolumeTypeBind && mount.Destination == "/dpanel" && !strings.HasSuffix(filepath.VolumeName(mount.Source), ":") {
			linkComposePath = filepath.Join(mount.Source, filepath.Base(workingDir))
		}
	}
	for _, mount := range dpanelContainerInfo.Mounts {
		if mount.Type == types.VolumeTypeBind && mount.Destination == "/dpanel/compose" && !strings.HasSuffix(filepath.VolumeName(mount.Source), ":") {
			linkComposePath = mount.Source
		}
	}
	if linkComposePath != "" {
		// 把软连的上级目录创建出来
		if _, err := os.Stat(linkComposePath); err != nil {
			_ = os.MkdirAll(filepath.Dir(linkComposePath), os.ModePerm)
		}
		if _, err := os.Readlink(linkComposePath); err != nil {
			// 当容器挂载了外部目录，创建时必须保证此目录有文件可以访问。否则相对目录会错误
			err := os.Symlink(workingDir, linkComposePath)
			slog.Debug("make compose symlink", "workdir", workingDir, "target", linkComposePath, "error", err)
		}
		workingDir = linkComposePath
	}

	slog.Info("compose get task", "workDir", workingDir)
	composeRun := self.LsItem(entity.Name)
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
		taskFileDir = filepath.Join(filepath.Dir(entity.Setting.Uri[0]))

		// 外部路径分两种，一种是原目录挂载，二是将Yaml文件放置到存储目录中（这种情况还是会当成挂载文件来对待）
		// 外部任务也可能是 dpanel 面板创建的，面板会将 compose 的文件地址与宿主机中的保持一致
		// 就会导致无法找到真正的文件，需要把文件中的主机路径，还是需要再添加一个软链接
		for _, item := range entity.Setting.Uri {
			yamlFilePath = append(yamlFilePath, item)
		}

		// 查找当前目录下是否有 dpanel-override 文件
		overrideFilePath := filepath.Join(taskFileDir, fmt.Sprintf("dpanel-%s-override.yaml", entity.Name))
		if len(yamlFilePath) > 0 && !function.InArray(yamlFilePath, overrideFilePath) {
			if _, err := os.Stat(overrideFilePath); err == nil {
				yamlFilePath = append(yamlFilePath, overrideFilePath)
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

	defaultEnvFileName := filepath.Join(taskFileDir, define.ComposeDefaultEnvFileName)
	var defaultEnvFileExists error

	// 如果任务中的环境变量值为空，则使用默认 .env 中的值填充
	// 默认情况下，不管 compose 中有没有指定 env_files 都会加载 .env 文件
	// 组合后最终的值，再写入回 .env 文件，写入时不要破坏 .env 原始文件内容，只替换变量

	if _, defaultEnvFileExists = os.Stat(defaultEnvFileName); defaultEnvFileExists == nil {
		if defaultEnvContent, err := os.ReadFile(defaultEnvFileName); err == nil {
			for _, s := range strings.Split(string(defaultEnvContent), "\n") {
				if name, value, exists := strings.Cut(s, "="); exists {
					if exists, i := function.IndexArrayWalk(entity.Setting.Environment, func(i docker.EnvItem) bool {
						if i.Name == name {
							return true
						}
						return false
					}); exists {
						// 以 .env 中的数据为优先，因为最后保存的时候会同步一份给 .env
						if entity.Setting.Environment[i].Value == "" {
							entity.Setting.Environment[i].Value = value
						}
					} else {
						entity.Setting.Environment = append(entity.Setting.Environment, docker.EnvItem{
							Name:  name,
							Value: value,
						})
					}
				}
			}
		}
	}

	if dpanelEnvContent, err := os.ReadFile(filepath.Join(taskFileDir, define.ComposeProjectEnvFileName)); err == nil {
		for _, s := range strings.Split(string(dpanelEnvContent), "\n") {
			if name, value, exists := strings.Cut(s, "="); exists {
				if exists, i := function.IndexArrayWalk(entity.Setting.Environment, func(i docker.EnvItem) bool {
					// 如果数据库中环境变量有值时，则不使用 .env 中的覆盖
					if i.Name == name {
						return true
					}
					return false
				}); exists {
					// .dpanel.env 中的数据强制覆盖到 .env 中
					if entity.Setting.Environment[i].Value == "" {
						entity.Setting.Environment[i].Value = value
					}
				} else {
					entity.Setting.Environment = append(entity.Setting.Environment, docker.EnvItem{
						Name:  name,
						Value: value,
					})
				}
			}
		}
	}

	if !function.IsEmptyArray(entity.Setting.Environment) {
		globalEnv := function.PluckArrayWalk(entity.Setting.Environment, func(i docker.EnvItem) (string, bool) {
			return fmt.Sprintf("%s=%s", i.Name, i.Value), true
		})
		err := os.MkdirAll(filepath.Dir(defaultEnvFileName), os.ModePerm)
		if err != nil {
			return nil, err
		}
		err = os.WriteFile(defaultEnvFileName, []byte(strings.Join(globalEnv, "\n")), 0666)
		// 环境变量只为生成 .env 文件，不能直接附加，可能会出来 环境变量中套用环境变量，产生值不对的情况
		//options = append(options, cli.WithEnv(globalEnv))
		options = append(options, cli.WithEnvFiles(defaultEnvFileName))
		options = append(options, cli.WithDotEnv)
	}

	projectName := fmt.Sprintf(define.ComposeProjectName, strings.ReplaceAll(entity.Name, "@", "-"))
	if entity.Setting.Type == accessor.ComposeTypeOutPath {
		// compose 项止名称不允许有大小写，但是compose的目录名可以包含特殊字符，这里统一用id进行区分
		// 如果是外部任务，则保持原有名称
		// 如果该任务已经运行，但是不包含面板dpanel-c前缀，则表示该任务并非是面板创建的
		// 只不过文件挂载到目录中，当成了挂载任务来对待
		// 需要特殊的处理一下 -p 参数
		// 此处还需要兼容包含 dpanel-c 前缀的外部任务
	}

	if !composeRun.IsDPanel {
		projectName = entity.Name
	}

	options = append(options, cli.WithName(projectName))

	// 最终Yaml需要用到原始的compose，创建一个原始的对象
	originalComposer, err := compose.NewCompose(options...)
	if err != nil {
		slog.Warn("compose get task ", "error", err)
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

func (self Compose) GetTaskContainer(taskName string) []*compose.ContainerResult {
	result := make([]*compose.ContainerResult, 0)
	if containerList, err := docker.Sdk.Client.ContainerList(docker.Sdk.Ctx, container.ListOptions{
		All: true,
	}); err == nil {
		result = function.PluckArrayWalk(containerList, func(item container.Summary) (*compose.ContainerResult, bool) {
			if _, ok := item.Labels[define.ComposeLabelProject]; !ok {
				return nil, false
			}
			if v, ok := item.Labels[define.ComposeLabelProject]; ok && taskName != "" && !strings.HasSuffix(v, taskName) {
				return nil, false
			}
			containerInfo, err := docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, item.ID)
			if err != nil {
				return nil, false
			}
			health := ""
			if containerInfo.State.Health != nil {
				health = containerInfo.State.Health.Status
			}
			return &compose.ContainerResult{
				Project: item.Labels[define.ComposeLabelProject],
				Name:    containerInfo.Name,
				Service: item.Labels[define.ComposeLabelService],
				Publishers: function.PluckArrayWalk(item.Ports, func(i container.Port) (compose.ContainerPublishersResult, bool) {
					return compose.ContainerPublishersResult{
						URL:           i.IP,
						TargetPort:    i.PrivatePort,
						PublishedPort: i.PublicPort,
						Protocol:      i.Type,
					}, true
				}),
				State:  item.State,
				Status: item.Status,
				Health: health,
			}, true
		})
	}
	return result
}

func (self Compose) GetDPanelProjectName(name string) string {
	return fmt.Sprintf(define.ComposeProjectName, strings.ReplaceAll(name, "@", "-"))
}
