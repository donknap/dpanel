package logic

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/compose-spec/compose-go/v2/cli"
	"github.com/compose-spec/compose-go/v2/dotenv"
	"github.com/compose-spec/compose-go/v2/paths"
	"github.com/compose-spec/compose-go/v2/types"
	"github.com/docker/docker/api/types/container"
	commonlogic "github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/agent/factor"
	"github.com/donknap/dpanel/common/service/compose"
	"github.com/donknap/dpanel/common/service/docker"
	types2 "github.com/donknap/dpanel/common/service/docker/types"
	serviceafs "github.com/donknap/dpanel/common/service/fs/afs"
	"github.com/donknap/dpanel/common/service/fs/dockerfs"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/define"
	"gorm.io/gorm"
)

var ErrMissingComposeFile = errors.New("compose file not found")

type Compose struct {
}

// 仅支持 id 和 运行中 compose 名称获取
func (self Compose) Get(key string) (*entity.Compose, error) {
	var composeRow *entity.Compose
	var runTask *compose.ProjectResult
	id, err := strconv.Atoi(key)
	if err == nil {
		composeRow, err = dao.Compose.Where(dao.Compose.ID.Eq(int32(id))).First()
		if err != nil {
			return nil, err
		}
		runTask = self.LsItem(composeRow.Name)
	} else {
		runTask = self.LsItem(key)
		if runTask == nil {
			return nil, gorm.ErrRecordNotFound
		}
		composeRow = &entity.Compose{
			Name:  key,
			Title: "",
			Setting: &accessor.ComposeSettingOption{
				Type:          accessor.ComposeTypeOutPath,
				DockerEnvName: docker.Sdk.DockerEnv.GetComposeEnvName(),
				Environment:   make([]types2.EnvItem, 0),
			},
		}
	}
	if composeRow.Setting == nil {
		return nil, errors.New("compose settings are required")
	}
	if composeRow.ID == 0 && composeRow.Setting.CreatedAt == "" {
		composeRow.Setting.CreatedAt = time.Now().Local().Format(time.DateTime)
		composeRow.Setting.UpdatedAt = composeRow.Setting.CreatedAt
	}

	if runTask != nil {
		composeRow.Setting.Status = runTask.Status
		if strings.HasPrefix(runTask.RunName, define.ComposeProjectPrefix) {
			composeRow.Setting.RunName = runTask.RunName
		}
		if composeRow.Setting.Type == accessor.ComposeTypeOutPath && len(composeRow.Setting.Uri) == 0 {
			composeRow.Setting.Uri = runTask.ConfigFileList
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
					item = function.PathSeparatorToSlash(item)
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
		slog.Warn("compose sync path not found", "error", err)
		return make(map[string]*entity.Compose)
	}

	var linkRealPath string
	// 如果是软链接，获取到实际指向的目录
	if fileInfo, err := os.Lstat(rootDir); err == nil && fileInfo.Mode()&os.ModeSymlink == os.ModeSymlink {
		if linkRealPath, err = os.Readlink(rootDir); err == nil {
			rootDir = linkRealPath
		}
	}

	slog.Info("compose find yaml in path", "path", rootDir, "link path", linkRealPath)

	findComposeList := make(map[string]*entity.Compose)
	pathList, err := os.ReadDir(rootDir)
	if err != nil {
		return findComposeList
	}

	for _, dirEntry := range pathList {
		if !dirEntry.IsDir() {
			continue
		}
		for _, suffix := range accessor.ComposeFileNameSuffix {
			relYamlFilePath := path.Join(dirEntry.Name(), suffix)
			name := strings.ToLower(dirEntry.Name())
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
						DockerEnvName: docker.Sdk.DockerEnv.GetComposeEnvName(),
					},
				}
				relOverrideYamlPath := path.Join(dirEntry.Name(), define.ComposeProjectDeployOverrideFileName)
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
	if _, err := os.Stat(rootDir); err != nil {
		return nil
	}

	// 循环任务，添加，清理任务
	composeList, _ := dao.Compose.Find()
	for _, dbComposeRow := range composeList {
		if find, ok := findComposeList[dbComposeRow.Name]; ok && find.Setting.DockerEnvName == dbComposeRow.Setting.DockerEnvName {
			if dbComposeRow.Setting.Type != accessor.ComposeTypeOutPath {
				dbComposeRow.Setting.Uri = find.Setting.Uri
				_ = dao.Compose.Save(dbComposeRow)
			}
			delete(findComposeList, dbComposeRow.Name)
		} else if dbComposeRow.Setting.DockerEnvName == dockerEnvName && (dbComposeRow.Setting.Type != accessor.ComposeTypeOutPath || dbComposeRow.Setting.IsDelegate()) {
			composeFile, _, err := dbComposeRow.Setting.GetUriPath(dbComposeRow.Name)
			if err != nil {
				return err
			}
			if _, err = os.Lstat(filepath.Dir(composeFile)); errors.Is(err, os.ErrNotExist) {
				if _, err = dao.Compose.Where(dao.Compose.ID.Eq(dbComposeRow.ID)).Delete(); err != nil {
					return err
				}
			} else if err != nil {
				return err
			}
		}
	}
	for _, running := range self.Ls() {
		if find, ok := findComposeList[running.Name]; ok {
			if len(running.ConfigFileList) == 0 {
				delete(findComposeList, running.Name)
				continue
			}
			composeFile := filepath.Join(rootDir, running.Name, path.Base(running.ConfigFileList[0]))
			if fileInfo, err := os.Stat(composeFile); err != nil || !fileInfo.Mode().IsRegular() {
				delete(findComposeList, running.Name)
				continue
			}
			find.Setting.Type = accessor.ComposeTypeOutPath
			find.Setting.Uri = running.ConfigFileList
			find.Setting.Delegate = function.Ptr(true)
		}
	}

	if !function.IsEmptyMap(findComposeList) {
		for _, item := range findComposeList {
			_ = dao.Compose.Create(item)
		}
	}

	return nil
}

func (self Compose) ComposeProjectOptionsFn(dbRow *entity.Compose) ([]cli.ProjectOptionsFn, error) {
	if dbRow == nil || dbRow.Setting == nil || !function.IsSafeString(dbRow.Name, function.StringRuleName) {
		return nil, errors.New("invalid compose task")
	}
	composeSetting := *dbRow.Setting
	composeSetting.Uri = make([]string, len(dbRow.Setting.Uri))
	for index, uri := range dbRow.Setting.Uri {
		composeSetting.Uri[index] = function.PathSeparatorToSlash(uri)
	}
	composeFile, overrideFile, err := composeSetting.GetUriPath(dbRow.Name)
	if err != nil {
		return nil, err
	}
	fileList := make([]string, 0, len(composeSetting.Uri)+1)
	for index, uri := range composeSetting.Uri {
		filePath := function.SlashPathToSystem(uri)
		if composeSetting.Type != accessor.ComposeTypeOutPath || composeSetting.IsDelegate() {
			filename := path.Base(uri)
			if index > 0 && composeSetting.IsDelegate() && filename == fmt.Sprintf(define.ComposeProjectDeployOverrideOutPathFileName, dbRow.Name) {
				filename = define.ComposeProjectDeployOverrideFileName
			}
			filePath = filepath.Join(filepath.Dir(composeFile), filename)
		}
		if !function.InArray(fileList, filePath) {
			fileList = append(fileList, filePath)
		}
	}
	if composeSetting.Type == accessor.ComposeTypeOutPath {
		if !function.InArray(fileList, overrideFile) {
			if _, err := os.Stat(overrideFile); err == nil {
				fileList = append(fileList, overrideFile)
			} else if !errors.Is(err, os.ErrNotExist) {
				return nil, err
			}
		}
	}
	fileList[0] = composeFile
	optionList := make([]cli.ProjectOptionsFn, 0, len(fileList)+2)
	for _, filePath := range fileList {
		optionList = append(optionList, compose.WithYamlPath(filePath))
	}
	optionList = append(optionList, func(options *cli.ProjectOptions) error {
		options.WorkingDir = filepath.Dir(composeFile)
		return nil
	})
	if dbRow.Setting.RunName != "" {
		optionList = append(optionList, cli.WithName(dbRow.Setting.RunName))
	} else {
		optionList = append(optionList, cli.WithName(dbRow.Name))
	}
	return optionList, nil
}

func (self Compose) GetTasker(dockerSdk *docker.Client, dbRow *entity.Compose) (taskResult *compose.Task, warning error, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	defer func() {
		if err == nil || errors.Is(err, ErrMissingComposeFile) || function.IsErrorMessage(err) {
			return
		}
		if function.ErrorHasKeyword(err, "invalid project name") {
			err = function.ErrorMessage(define.ErrorMessageComposeNameInvalid)
		} else {
			err = function.ErrorMessage(define.ErrorMessageComposeParseYamlIncorrect, "error", errors.Join(warning, err).Error())
		}
	}()

	optionList, err := self.ComposeProjectOptionsFn(dbRow)
	if err != nil {
		return nil, nil, err
	}
	projectOptions, err := cli.NewProjectOptions(nil, optionList...)
	if err != nil {
		return nil, nil, err
	}
	taskEnv := append(make([]types2.EnvItem, 0, len(dbRow.Setting.Environment)), dbRow.Setting.Environment...)
	task := &compose.Task{ComposeFiles: projectOptions.ConfigPaths}
	defaultEnvPath, defaultEnvContent, err := task.GetDefaultEnv()
	if err != nil {
		return nil, nil, err
	}
	if defaultEnvContent != nil {
		fileEnv, err := dotenv.UnmarshalWithLookup(string(defaultEnvContent), nil)
		if err != nil {
			return task, nil, function.ErrorMessage(define.ErrorMessageComposeEnvInvalid, "error", err.Error())
		}
		fileInfo, err := os.Stat(defaultEnvPath)
		if err != nil {
			return nil, nil, err
		}
		updatedAt, _ := time.ParseInLocation(time.DateTime, dbRow.Setting.UpdatedAt, time.Local)
		envIndexMap := make(map[string]int, len(taskEnv))
		for envIndex, item := range taskEnv {
			envIndexMap[item.Name] = envIndex
		}
		envNameList := make([]string, 0, len(fileEnv))
		for name := range fileEnv {
			envNameList = append(envNameList, name)
		}
		sort.Strings(envNameList)
		for _, name := range envNameList {
			if envIndex, ok := envIndexMap[name]; ok {
				if fileInfo.ModTime().Truncate(time.Second).After(updatedAt) {
					taskEnv[envIndex].Value = fileEnv[name]
				}
			} else {
				taskEnv = append(taskEnv, types2.EnvItem{
					Name:  name,
					Value: fileEnv[name],
					Rule:  &types2.EnvValueRule{Kind: types2.EnvValueRuleInEnvFile},
				})
			}
		}
	}
	task.Environment = taskEnv
	task.Project = &types.Project{}
	fileInfo, err := os.Stat(task.ComposeFiles[0])
	if errors.Is(err, os.ErrNotExist) {
		return task, nil, ErrMissingComposeFile
	}
	if err != nil {
		return nil, nil, err
	}
	if !fileInfo.Mode().IsRegular() {
		return nil, nil, fmt.Errorf("compose file %q is not a regular file", task.ComposeFiles[0])
	}
	if fileInfo.Size() == 0 {
		return task, nil, ErrMissingComposeFile
	}
	if err := cli.WithEnv(function.PluckArrayWalk(taskEnv, func(item types2.EnvItem) (string, bool) {
		return fmt.Sprintf("%s=%s", item.Name, item.Value), true
	}))(projectOptions); err != nil {
		return task, nil, err
	}
	tasker, warning, err := compose.NewCompose(ctx, dockerSdk, projectOptions)
	if err != nil {
		return task, warning, err
	}
	tasker.Environment = taskEnv
	if dbRow.Setting.Type == accessor.ComposeTypeOutPath {
		tasker.WorkingDir = path.Dir(function.PathSeparatorToSlash(dbRow.Setting.Uri[0]))
	} else if containerDir, hostDir := self.ComposeDir(dbRow.Setting.DockerEnvName); hostDir != "" {
		relativeDir, err := filepath.Rel(function.SlashPathToSystem(containerDir), tasker.Project.WorkingDir)
		if err != nil {
			return nil, warning, err
		}
		tasker.WorkingDir = path.Join(hostDir, filepath.ToSlash(relativeDir))
	}
	return tasker, warning, nil
}

func (self Compose) ValidateRelativeBindMounts(tasker *compose.Task) error {
	if tasker == nil || !strings.HasPrefix(filepath.Clean(tasker.WorkingDir), define.ComposeVolumeCheckPath+string(filepath.Separator)) {
		return nil
	}
	for serviceName, service := range tasker.Project.AllServices() {
		for _, volume := range service.Volumes {
			if volume.Type != types.VolumeTypeBind {
				continue
			}
			source := paths.ExpandUser(volume.Source)
			if source == "" || filepath.IsAbs(source) || paths.IsWindowsAbs(source) {
				continue
			}
			return function.ErrorMessage(
				define.ErrorMessageComposeRelativeBindOnVolume,
				serviceName, volume.Source, volume.Target,
			)
		}
	}
	return nil
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

func (self Compose) ComposeDir(dockerEnvName string) (containerDir, hostDir string) {
	containerDir, err := filepath.Abs(storage.Local{}.GetComposePath(dockerEnvName))
	if err != nil {
		return "", ""
	}
	containerDir = filepath.ToSlash(containerDir)
	if !function.IsRunInDocker() {
		return containerDir, containerDir
	}

	dpanelInfo := commonlogic.Setting{}.GetDPanelInfo()
	mountDestination := ""
	for _, mount := range dpanelInfo.ContainerInfo.Mounts {
		destination := filepath.Clean(mount.Destination)
		if !filepath.IsAbs(destination) || len(destination) <= len(mountDestination) {
			continue
		}
		relativeDir, err := filepath.Rel(destination, containerDir)
		if err != nil || !filepath.IsLocal(relativeDir) {
			continue
		}
		mountDestination = destination
		hostDir = ""
		source := function.PathSeparatorToSlash(mount.Source)
		switch mount.Type {
		case "bind":
			if filepath.IsAbs(source) || paths.IsWindowsAbs(source) {
				hostDir = source
				if relativeDir != "." {
					hostDir = path.Join(source, filepath.ToSlash(relativeDir))
				}
			}
		case "volume":
			hostDir = path.Join(define.ComposeVolumeCheckPath, dockerEnvName)
		}
	}
	return containerDir, hostDir
}

// SyncLocalDir aligns a host path with a directory visible to the panel.
func (self Compose) SyncLocalDir(sourceDir, targetDir string) error {
	if filepath.Clean(sourceDir) == filepath.Clean(targetDir) {
		return nil
	}
	if targetInfo, err := os.Stat(targetDir); err == nil && targetInfo.IsDir() {
		if sourceInfo, err := os.Stat(sourceDir); err == nil && os.SameFile(sourceInfo, targetInfo) {
			return nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(targetDir), os.ModePerm); err != nil {
		return fmt.Errorf("prepare compose path %q: %w", targetDir, err)
	}
	if existing, err := os.Readlink(targetDir); err == nil {
		if filepath.Clean(existing) == filepath.Clean(sourceDir) {
			return nil
		}
		return fmt.Errorf("compose path %q already links to %q", targetDir, existing)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("compose path %q already exists: %w", targetDir, err)
	}
	if err := os.Symlink(sourceDir, targetDir); err != nil {
		return fmt.Errorf("link compose directory %q: %w", targetDir, err)
	}
	return nil
}

func (self Compose) DeleteLocalLink(sourceDir, targetDir string) error {
	if filepath.Clean(sourceDir) == filepath.Clean(targetDir) {
		return nil
	}
	fileInfo, err := os.Lstat(targetDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if fileInfo.Mode()&os.ModeSymlink == 0 {
		return nil
	}
	linkDir, err := os.Readlink(targetDir)
	if err != nil {
		return err
	}
	if !filepath.IsAbs(linkDir) {
		linkDir = filepath.Join(filepath.Dir(targetDir), linkDir)
	}
	if filepath.Clean(linkDir) != filepath.Clean(sourceDir) {
		return function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted)
	}
	if err = os.Remove(targetDir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// SyncRemoteDir copies a panel Compose directory to the selected remote Docker filesystem.
func (self Compose) SyncRemoteDir(ctx context.Context, dockerSdk *docker.Client, sourceDir, targetDir string, hostTarget bool) (err error) {
	var fileSystem serviceafs.Fs
	if hostTarget {
		if !path.IsAbs(targetDir) || path.Clean(targetDir) != targetDir {
			return fmt.Errorf("invalid remote compose target %q", targetDir)
		}
		var hostExplorerName string
		hostExplorerName, err = factor.NewExplorerHost(dockerSdk, factor.HostCreateOption{})
		if err != nil {
			return err
		}
		defer func() { err = errors.Join(err, factor.Destroy(dockerSdk, hostExplorerName)) }()
		fileSystem, err = dockerfs.New(
			dockerfs.WithDockerSdk(dockerSdk),
			dockerfs.WithProxyContainer(hostExplorerName),
			dockerfs.WithRoot(factor.HostExplorerMountPath),
		)
	} else {
		var relativeDir string
		relativeDir, err = filepath.Rel(storage.Local{}.GetStorageLocalPath(), sourceDir)
		if err != nil {
			return err
		}
		targetDir = path.Join("/", filepath.ToSlash(relativeDir))
		fileSystem, err = (commonlogic.Explorer{}).Afs(ctx,
			commonlogic.ExplorerMountPoint(commonlogic.ExplorerMountTypeContainer+":"+factor.ExplorerName), dockerSdk,
		)
	}
	if err != nil {
		return err
	}
	return fileSystem.Import([]serviceafs.TransferFile{{Source: sourceDir, Target: targetDir}})
}

func (self Compose) DeleteRemoteDir(ctx context.Context, dockerSdk *docker.Client, sourceDir string) error {
	composePath := storage.Local{}.GetComposePath(dockerSdk.DockerEnv.GetComposeEnvName())
	relativeDir, err := filepath.Rel(composePath, sourceDir)
	if err != nil || relativeDir == "." || !filepath.IsLocal(relativeDir) {
		return function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted)
	}
	fileSystem, err := (commonlogic.Explorer{}).Afs(ctx,
		commonlogic.ExplorerMountPoint(commonlogic.ExplorerMountTypeContainer+":"+factor.ExplorerName), dockerSdk,
	)
	if err != nil {
		return err
	}
	defer fileSystem.Destroy()
	targetDir := path.Join("/", filepath.Base(composePath), filepath.ToSlash(relativeDir))
	if err = fileSystem.RemoveAll(targetDir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
