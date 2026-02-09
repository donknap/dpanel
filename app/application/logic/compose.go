package logic

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/compose-spec/compose-go/v2/cli"
	"github.com/compose-spec/compose-go/v2/dotenv"
	"github.com/compose-spec/compose-go/v2/loader"
	"github.com/compose-spec/compose-go/v2/types"
	"github.com/docker/docker/api/types/container"
	"github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/compose"
	"github.com/donknap/dpanel/common/service/docker"
	types2 "github.com/donknap/dpanel/common/service/docker/types"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/define"
)

var ComposeFileNameSuffix = []string{
	"docker-compose.yml", "docker-compose.yaml",
	"compose.yml", "compose.yaml",
}

type Compose struct {
}

func (self Compose) Get(key string) (*entity.Compose, error) {
	var composeRow *entity.Compose

	if id, err := strconv.Atoi(key); err == nil {
		composeRow, _ = dao.Compose.Where(dao.Compose.ID.Eq(int32(id))).First()
	} else {
		composeRow, _ = dao.Compose.Where(dao.Compose.Name.Eq(key)).First()
	}

	if composeRow == nil {
		composeRow = &entity.Compose{
			Name:  key,
			Title: "",
			Setting: &accessor.ComposeSettingOption{
				Type:          accessor.ComposeTypeOutPath,
				DockerEnvName: docker.Sdk.Name,
				Environment:   make([]types2.EnvItem, 0),
			},
		}
	}

	runTaskList := self.Ls()
	if v, _, ok := function.PluckArrayItemWalk(runTaskList, func(item *compose.ProjectResult) bool {
		return item.Name == composeRow.Name
	}); ok {
		composeRow.Setting.Status = v.Status
		composeRow.Setting.UpdatedAt = v.UpdatedAt.Local().Format(time.DateTime)
		if strings.HasPrefix(v.RunName, define.ComposeProjectPrefix) {
			composeRow.Setting.RunName = v.RunName
		}
		if composeRow.Setting.Type == accessor.ComposeTypeOutPath {
			if !v.CanManage {
				composeRow.Setting.Type = accessor.ComposeTypeDangling
			}
			composeRow.Setting.Uri = v.ConfigFileList
		}
	} else {
		composeRow.Setting.Status = accessor.ComposeStatusWaiting
	}

	return composeRow, nil
}

// Ps 获取所有 compose 下的容器
func (self Compose) Ps(projectNameList ...string) []*compose.ContainerResult {
	result := make([]*compose.ContainerResult, 0)

	runComposeList := self.Ls()
	// 如果为空，则获取所有的 compose 容器
	if function.IsEmptyArray(projectNameList) {
		projectNameList = function.PluckArrayWalk(runComposeList, func(item *compose.ProjectResult) (string, bool) {
			return item.Name, true
		})
	}
	for _, name := range projectNameList {
		project, _, ok := function.PluckArrayItemWalk(runComposeList, func(project *compose.ProjectResult) bool {
			return project.Name == name
		})
		if !ok {
			return result
		}
		for _, summary := range project.ContainerList {
			if containerInfo, err := docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, summary.Container.ID); err == nil {
				health := ""
				if containerInfo.State.Health != nil {
					health = containerInfo.State.Health.Status
				}
				result = append(result, &compose.ContainerResult{
					Project: project.Name,
					Name:    containerInfo.Name,
					Service: summary.Service,
					Publishers: function.PluckArrayWalk(summary.Container.Ports, func(i container.Port) (compose.ContainerPublishersResult, bool) {
						publicPort := i.PublicPort
						if publicPort == 0 {
							publicPort = i.PrivatePort
						}
						return compose.ContainerPublishersResult{
							URL:           i.IP,
							TargetPort:    i.PrivatePort,
							PublishedPort: publicPort,
							Protocol:      i.Type,
						}, true
					}),
					State:  summary.Container.State,
					Status: summary.Container.Status,
					Health: health,
				})
			}
		}
	}
	return result
}

func (self Compose) Ls() []*compose.ProjectResult {
	composeGroupContainerList := make(map[string][]container.Summary)
	if containerList, err := docker.Sdk.Client.ContainerList(docker.Sdk.Ctx, container.ListOptions{
		All: true,
	}); err == nil {
		for _, summary := range containerList {
			projectName, ok := summary.Labels[define.ComposeLabelProject]
			if !ok {
				continue
			}
			if strings.HasPrefix(projectName, define.ComposeProjectPrefix) {
				// 将带前缀的任务也当成普通任务进行分组，防止丢失，后续更新全部会恢复到原始名称
				_, projectName, _ = strings.Cut(projectName, define.ComposeProjectPrefix)
			}
			group, ok := composeGroupContainerList[projectName]
			if !ok {
				group = []container.Summary{}
			}
			group = append(group, summary)
			composeGroupContainerList[projectName] = group
		}
	}

	result := make([]*compose.ProjectResult, 0)
	for name, containerList := range composeGroupContainerList {
		task := &compose.ProjectResult{
			Name:           name,
			ConfigFileList: make([]string, 0),
			ContainerList:  make([]compose.TaskResultRunContainerResult, 0),
			Status:         "",
		}

		status := make([]string, 0)
		for _, summary := range containerList {
			if configFiles, ok := summary.Labels[define.ComposeLabelConfigFiles]; ok {
				task.ConfigFileList = append(task.ConfigFileList, function.PluckArrayWalk(strings.Split(configFiles, ","), func(item string) (string, bool) {
					return item, !function.InArray(task.ConfigFileList, item)
				})...)
			}
			status = append(status, summary.State)
			if v := time.Unix(summary.Created, 0); v.After(task.UpdatedAt) {
				task.UpdatedAt = v
			}
			task.ContainerList = append(task.ContainerList, compose.TaskResultRunContainerResult{
				Container:  summary,
				ConfigHash: summary.Labels[define.ComposeLabelConfigHash],
				Service:    summary.Labels[define.ComposeLabelService],
			})
			task.RunName = summary.Labels[define.ComposeLabelProject]
		}

		function.CombinedArrayValueCount(status, func(key string, count int) {
			if task.Status != "" {
				task.Status += ", "
			}
			task.Status += fmt.Sprintf("%s(%d)", key, count)
		})

		// 面板为了对齐宿主机的目录，实际部署的时候可能 config_files 的路径并不是 /dpanel/compose
		// 直接使用 Name 到 /dpanel/compose 查找，如果存在，直接将 config_files 重定向到面板路径中
		// 如果不存在，再直接查找，属于外部任务
		// 如果不存在，属于 dangling 任务，无法管理
		tryFind := function.PluckArrayWalk(task.ConfigFileList, func(file string) (string, bool) {
			return filepath.Join("/dpanel", task.Name, filepath.Base(file)), true
		})
		if function.FileExists(tryFind...) {
			task.ConfigFileList = tryFind
		}
		if !task.CanManage && function.FileExists(task.ConfigFileList...) {
			task.CanManage = true
		}
		task.ConfigFiles = strings.Join(task.ConfigFileList, ",")
		result = append(result, task)
	}
	return result
}

func (self Compose) LsItem(name string) *compose.ProjectResult {
	var result *compose.ProjectResult
	for _, item := range self.Ls() {
		if item.Name == name {
			result = item
			break
		}
	}
	return result
}

// FindPathTask 查询 docker 环境下 compose 目录下的所有任务
func (self Compose) FindPathTask(rootDir string) map[string]*entity.Compose {
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
						DockerEnvName: docker.Sdk.Name,
					},
				}
				relOverrideYamlPath := filepath.Join(path.Name(), define.ComposeProjectDeployOverrideFileName)
				if _, err = os.Stat(filepath.Join(rootDir, relOverrideYamlPath)); err == nil {
					findRow.Setting.Uri = append(findRow.Setting.Uri, relOverrideYamlPath)
				}
				findComposeList[name] = findRow
				break
			}
		}
	}
	return findComposeList
}

// Sync 同步当前挂载目录中的 compose
func (self Compose) Sync(dockerEnvName string) error {
	rootDir := storage.Local{}.GetComposePath(dockerEnvName)
	findComposeList := self.FindPathTask(rootDir)

	// 循环任务，添加，清理任务
	composeList, _ := dao.Compose.Find()
	for _, dbComposeRow := range composeList {
		if find, ok := findComposeList[dbComposeRow.Name]; ok && find.Setting.DockerEnvName == dbComposeRow.Setting.DockerEnvName {
			// 终始以目录下的实际文件为准
			dbComposeRow.Setting.Uri = find.Setting.Uri
			_ = dao.Compose.Save(dbComposeRow)
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

func (self Compose) ComposeProjectOptionsFn(dbRow *entity.Compose) []cli.ProjectOptionsFn {
	workingDir := dbRow.Setting.GetWorkingDir()

	// 如果面板的 /dpanel 挂载到了宿主机，则重新设置 workDir
	linkComposePath := ""
	dpanelInfo := logic.Setting{}.GetDPanelInfo()

	for i, mount := range dpanelInfo.ContainerInfo.Mounts {
		if mount.Type != types.VolumeTypeBind {
			continue
		}
		if v, ok := function.PathConvertWinPath2Unix(mount.Source); ok {
			dpanelInfo.ContainerInfo.Mounts[i].Source = filepath.Join("/", "mnt", "host", v)
		}
	}

	for _, mount := range dpanelInfo.ContainerInfo.Mounts {
		if mount.Type == types.VolumeTypeBind && mount.Destination == "/dpanel" && !strings.HasSuffix(filepath.VolumeName(mount.Source), ":") {
			linkComposePath = filepath.Join(mount.Source, filepath.Base(workingDir))
		}
	}
	// 如果开启了独立目录，获取挂载目录也应该只取对应的的
	mountComposePath := "/dpanel/compose"

	if docker.Sdk.DockerEnv.EnableComposePath {
		mountComposePath = filepath.Join("/", "dpanel", "compose-"+dbRow.Setting.DockerEnvName)
	}

	for _, mount := range dpanelInfo.ContainerInfo.Mounts {
		if mount.Type == types.VolumeTypeBind && mount.Destination == mountComposePath && !strings.HasSuffix(filepath.VolumeName(mount.Source), ":") {
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
			err = os.Symlink(workingDir, linkComposePath)
			if err != nil {
				slog.Debug("db compose make path symlink", "workdir", workingDir, "target", linkComposePath, "error", err)
			}
		}
		workingDir = linkComposePath
	}

	slog.Info("db compose ", "workDir", workingDir)

	taskFileDir := filepath.Join(workingDir, filepath.Dir(dbRow.Setting.Uri[0]))

	yamlFilePath := make([]string, 0)

	if dbRow.Setting.Type == accessor.ComposeTypeOutPath {
		taskFileDir = filepath.Join(filepath.Dir(dbRow.Setting.Uri[0]))

		// 外部路径分两种，一种是原目录挂载，二是将Yaml文件放置到存储目录中（这种情况还是会当成挂载文件来对待）
		// 外部任务也可能是 dpanel 面板创建的，面板会将 compose 的文件地址与宿主机中的保持一致
		// 就会导致无法找到真正的文件，需要把文件中的主机路径，还是需要再添加一个软链接
		for _, item := range dbRow.Setting.Uri {
			yamlFilePath = append(yamlFilePath, item)
		}

		// 查找当前目录下是否有 dpanel-override 文件
		overrideFilePath := filepath.Join(taskFileDir, fmt.Sprintf(define.ComposeProjectDeployOverrideOutPathFileName, dbRow.Name))
		if len(yamlFilePath) > 0 && !function.InArray(yamlFilePath, overrideFilePath) {
			if _, err := os.Stat(overrideFilePath); err == nil {
				yamlFilePath = append(yamlFilePath, overrideFilePath)
			}
		}
	} else {
		for _, item := range dbRow.Setting.Uri {
			yamlFilePath = append(yamlFilePath, filepath.Join(taskFileDir, filepath.Base(item)))
		}
	}

	options := make([]cli.ProjectOptionsFn, 0)
	for _, path := range yamlFilePath {
		options = append(options, compose.WithYamlPath(path))
	}

	if defaultEnvPath, defaultEnvContent, err := dbRow.Setting.GetDefaultEnv(); err == nil && defaultEnvContent != nil {
		options = append(options, cli.WithEnvFiles(defaultEnvPath))
	}
	options = append(options, cli.WithDotEnv)
	// 始终以提交上来的环境变量（包含 .env 文件），.env 的内容仅在编辑任务的时候会覆盖写入
	globalEnv := function.PluckArrayWalk(dbRow.Setting.Environment, func(i types2.EnvItem) (string, bool) {
		if i.Rule != nil && i.Rule.IsInEnvFile() {
			// 如果变量属于 .env 文件，则不主动附加，而是通过上面的 withEnvFile 进行附加
			return "", false
		}
		return fmt.Sprintf("%s=%s", i.Name, i.Value), true
	})
	options = append(options, cli.WithEnv(globalEnv))

	if dbRow.Setting.RunName != "" {
		options = append(options, cli.WithName(dbRow.Setting.RunName))
	} else {
		options = append(options, cli.WithName(dbRow.Name))
	}

	return options
}

func (self Compose) GetTasker(dbRow *entity.Compose) (*compose.Task, error, error) {
	options := self.ComposeProjectOptionsFn(dbRow)
	options = append(options, cli.WithLoadOptions(func(options *loader.Options) {
		options.SkipValidation = true
	}))
	task, warning, err := compose.NewCompose(options...)
	if err != nil {
		slog.Warn("compose get task ", "error", err)
		return nil, nil, err
	}
	return task, warning, nil
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

func (self Compose) getDPanelProjectName(name string) string {
	return fmt.Sprintf(define.ComposeProjectName, strings.ReplaceAll(name, "@", "-"))
}

func (self Compose) ParseEnvItemValue(env []types2.EnvItem) ([]types2.EnvItem, error) {
	envMap, err := dotenv.UnmarshalWithLookup(strings.Join(function.PluckArrayWalk(env, func(item types2.EnvItem) (string, bool) {
		return item.String(), true
	}), "\n"), nil)
	if err != nil {
		return nil, err
	}
	return function.PluckArrayWalk(env, func(item types2.EnvItem) (types2.EnvItem, bool) {
		if v, ok := envMap[item.Name]; ok {
			item.Value = v
		}
		return item, true
	}), nil
}
