package plugin

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"strings"
	"sync"

	"github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/compose"
	"github.com/donknap/dpanel/common/service/docker"
	builder "github.com/donknap/dpanel/common/service/docker/container"
	"github.com/donknap/dpanel/common/service/docker/types"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/google/uuid"
	"github.com/we7coreteam/registry-go-sdk"
)

const (
	DockerRootMountPath   = "/mnt_docker"
	ExplorerName          = "dpanel-plugin-explorer"
	HostExplorerMountPath = "/mnt_host"
	MonitorName           = "dpanel-plugin-monitor"
)

type CreateOption struct {
	Init                     bool               `json:"-"`
	RandomProxyContainerName bool               `json:"-"`
	ContainerName            string             `json:"containerName"`
	HostPID                  bool               `json:"hostPid"`
	MountDockerRoot          bool               `json:"mountDockerRoot"`
	MountHostRoot            bool               `json:"mountHostRoot"`
	Volumes                  []types.VolumeItem `json:"volumes"`
	VolumesFrom              []string           `json:"volumesFrom"`
	Command                  []string           `json:"command"`
	WorkingDir               string             `json:"workingDir"`
	Hash                     string             `json:"hash"`
	compose.ExtService
}

func NewPlugin(dockerSdk *docker.Client, name string, option CreateOption) (*Plugin, error) {
	p := &Plugin{
		dockerSdk:     dockerSdk,
		Name:          name,
		containerName: name,
		initialized:   option.Init,
	}
	if option.ContainerName != "" {
		p.containerName = option.ContainerName
	}
	if option.Init && option.MountDockerRoot {
		info, err := dockerSdk.Client.Info(dockerSdk.Ctx)
		if err != nil {
			return nil, fmt.Errorf("get Docker root directory: %w", err)
		}
		if info.DockerRootDir == "" {
			return nil, errors.New("Docker root directory is empty")
		}
		option.Volumes = append(option.Volumes, types.VolumeItem{
			Host:       info.DockerRootDir,
			Dest:       DockerRootMountPath,
			Type:       "bind",
			Permission: "readonly",
		})
	}
	if option.Init && option.MountHostRoot {
		option.Volumes = append(option.Volumes, types.VolumeItem{
			Host: "/",
			Dest: HostExplorerMountPath,
			Type: "bind",
		})
	}
	var asset embed.FS
	if v, ok := storage.Cache.Get(storage.CacheKeyAsset); ok {
		asset = v.(embed.FS)
	} else {
		return nil, define.ErrorAssetEmpty
	}
	p.asset = asset

	yamlTpl, err := asset.ReadFile("asset/plugin/" + name + "/compose.yaml")
	option.Hash = function.Sha256Struct(struct {
		Option   CreateOption
		Template string
	}{Option: option, Template: string(yamlTpl)})
	tpl := template.New(name)
	tpl.Funcs(template.FuncMap{
		"unescaped": func(s string) template.HTML {
			return template.HTML(s)
		},
	})
	parser, err := tpl.Parse(string(yamlTpl))
	if err != nil {
		return nil, err
	}
	buffer := new(bytes.Buffer)
	templateOption := function.StructToMap(option)
	if option.HostPID {
		templateOption["pidMode"] = "host"
	}
	err = parser.Execute(buffer, templateOption)
	if err != nil {
		return nil, err
	}
	slog.Debug("plugin parse yaml result", "yaml", buffer.String())
	p.composeTask, _, err = compose.NewCompose(compose.WithYamlContent(buffer.String()))
	if err != nil {
		return nil, err
	}
	_ = os.RemoveAll(p.composeTask.Project.WorkingDir)

	// 镜像准备，如果小于服务启动时间则重新导入
	service, serviceExt, err := p.composeTask.GetService(name)
	if err != nil {
		return nil, err
	}

	if option.Init && option.RandomProxyContainerName {
		p.containerName = fmt.Sprintf("%s-%s", name, uuid.NewString())
	} else if option.ContainerName != "" {
		p.containerName = option.ContainerName
	} else if service.ContainerName != "" {
		p.containerName = service.ContainerName
	} else {
		p.containerName = name
	}
	if !option.Init {
		return p, nil
	}

	dockerVersion, _ := dockerSdk.Client.ServerVersion(dockerSdk.Ctx)
	if imageTarUrl, ok := serviceExt.ImageTar[dockerVersion.Arch]; ok {
		if imageTarUrl != "" && strings.HasPrefix(imageTarUrl, "asset/plugin") {
			p.imagePath = imageTarUrl
		}
	}

	if serviceExt.ImageProxy != nil {
		imageNameDetail := function.ImageTag(service.Image)

		for _, proxy := range serviceExt.ImageProxy {
			reg := registry.New(
				registry.WithServer(proxy, "", ""),
			)
			if ok, _, err := reg.Client().ManifestExist(imageNameDetail.BaseName, imageNameDetail.Version); err == nil && ok {
				imageNameDetail.Registry = proxy
				break
			}
		}

		reader, err := dockerSdk.Client.ImagePull(dockerSdk.Ctx, imageNameDetail.Uri(), image.PullOptions{})
		if err != nil {
			slog.Debug("plugin pull image", "image", imageNameDetail.Uri(), "error", err)
			return nil, err
		}
		defer func() {
			_ = reader.Close()
		}()
		_, err = io.Copy(os.Stdout, reader)
		if err != nil {
			return nil, err
		}

		err = dockerSdk.Client.ImageTag(dockerSdk.Ctx, imageNameDetail.Uri(), service.Image)
		if err != nil {
			return nil, err
		}
		imageInfo, err := dockerSdk.Client.ImageInspect(dockerSdk.Ctx, service.Image)
		if err != nil {
			return nil, err
		}
		p.imageID = imageInfo.ID
		p.imagePath = ""

	}

	return p, nil
}

type Plugin struct {
	Name          string
	containerName string
	containerID   string
	imageID       string
	imagePath     string
	initialized   bool
	mu            sync.Mutex
	dockerSdk     *docker.Client
	composeTask   *compose.Task
	asset         embed.FS
}

func (self *Plugin) Create() error {
	if !self.initialized {
		return errors.New("plugin is not initialized")
	}
	mutex := storage.NewMutex(fmt.Sprintf(storage.CacheKeyPluginLifecycleLock, self.dockerSdk.Name, self.containerName))
	mutex.Lock()
	defer mutex.Unlock()

	return self.create()
}

func (self *Plugin) create() error {
	service, serviceExt, err := self.composeTask.GetService(self.Name)
	if err != nil {
		return err
	}
	if err = self.prepareImage(service.Image); err != nil {
		return err
	}
	networkMode := container.NetworkMode(service.NetworkMode)
	if networkMode == "" {
		networkMode = network.NetworkDefault
	}

	containerInfo, err := self.dockerSdk.Client.ContainerInspect(self.dockerSdk.Ctx, self.containerName)
	if err == nil {
		self.containerID = containerInfo.ID
		slog.Info("plugin create", "name", self.Name, "id", containerInfo.ID)
		hashMatched := containerInfo.Config != nil &&
			containerInfo.Config.Labels[define.DPanelLabelContainerHash] == service.Labels[define.DPanelLabelContainerHash]
		imageMatched := self.imageID == "" || containerInfo.Image == self.imageID
		if containerInfo.State != nil && !containerInfo.State.Restarting && hashMatched && imageMatched {
			if containerInfo.State.Running {
				return nil
			}
			if err = self.dockerSdk.Client.ContainerStart(self.dockerSdk.Ctx, containerInfo.ID, container.StartOptions{}); err == nil {
				return nil
			}
		}
	}

	err = self.close(false)
	if err != nil {
		return err
	}

	self.mu.Lock()
	defer self.mu.Unlock()

	options := []builder.Option{
		builder.WithImage(service.Image),
		builder.WithContainerName(self.containerName),
		builder.WithHostname(self.containerName),
		builder.WithNetworkMode(networkMode),
		builder.WithExtraHosts(types.ValueItem{
			Name:  "host.dpanel.local",
			Value: "host-gateway",
		}),
		builder.WithStdioKeepAlive(true),
		builder.WithLabel(function.PluckMapWalkArray(service.Labels, func(key string, value string) (types.ValueItem, bool) {
			return types.ValueItem{
				Name:  key,
				Value: value,
			}, true
		})...),
		builder.WithPrivileged(service.Privileged),
		builder.WithSecurityOpt(service.SecurityOpt...),
		builder.WithReadonlyRootfs(service.ReadOnly),
		builder.WithCapDrop(service.CapDrop...),
		builder.WithCap(service.CapAdd...),
		builder.WithEnv(function.PluckMapWalkArray(service.Environment, func(name string, value *string) (types.EnvItem, bool) {
			if value == nil {
				return types.EnvItem{Name: name}, true
			}
			return types.EnvItem{Name: name, Value: *value}, true
		})...),
		builder.WithRestartPolicy(&types.RestartPolicy{
			Name: service.Restart,
		}),
		builder.WithCgroupnsMode(container.CgroupnsMode(service.Cgroup)),
		builder.WithPid(service.Pid),
		builder.WithVolumesFromContainerName(serviceExt.External.VolumesFrom...),
		builder.WithCommand(service.Command),
		builder.WithWorkDir(service.WorkingDir),
	}

	volumes := make([]types.VolumeItem, 0, len(service.Volumes)+len(serviceExt.External.Volumes))
	volumeIndex := make(map[string]int, cap(volumes))
	appendVolume := func(item types.VolumeItem) {
		if index, ok := volumeIndex[item.Dest]; ok {
			volumes[index] = item
			return
		}
		volumeIndex[item.Dest] = len(volumes)
		volumes = append(volumes, item)
	}
	for _, item := range service.Volumes {
		permission := "write"
		if item.ReadOnly {
			permission = "readonly"
		}
		appendVolume(types.VolumeItem{
			Host:       item.Source,
			Dest:       item.Target,
			Permission: permission,
		})
	}

	for _, item := range serviceExt.External.Volumes {
		path := strings.Split(item, ":")
		appendVolume(types.VolumeItem{
			Host:       path[0],
			Dest:       path[1],
			Permission: "write",
		})
	}
	if len(volumes) > 0 {
		options = append(options, builder.WithVolume(volumes...))
	}

	b, err := builder.New(self.dockerSdk, options...)
	if err != nil {
		return err
	}
	containerID, err := b.Execute()
	if containerID != "" {
		self.containerID = containerID
	}
	if err != nil {
		return err
	}

	err = self.dockerSdk.Client.ContainerStart(self.dockerSdk.Ctx, containerID, container.StartOptions{})
	if err != nil {
		return err
	}

	function.Wait(self.dockerSdk.Ctx, containerID, func(v string) bool {
		if info, err := self.dockerSdk.Client.ContainerInspect(self.dockerSdk.Ctx, v); err == nil && info.State.Running {
			return true
		}
		return false
	})

	return nil
}

func (self *Plugin) prepareImage(imageName string) error {
	if self.imagePath == "" {
		return nil
	}
	imageLock := storage.NewMutex(fmt.Sprintf(storage.CacheKeyPluginImageLock, self.dockerSdk.Name, self.Name))
	resetImage := imageLock.TryLock()
	_, err := self.dockerSdk.Client.ImageInspect(self.dockerSdk.Ctx, imageName)
	if err == nil && !resetImage {
		return nil
	}
	if err != nil && !errdefs.IsNotFound(err) {
		if resetImage {
			imageLock.Unlock()
		}
		return err
	}
	if err == nil {
		if err = self.close(true); err != nil {
			imageLock.Unlock()
			return err
		}
	}

	imageFile, err := self.asset.Open(self.imagePath)
	if err != nil {
		if resetImage {
			imageLock.Unlock()
		}
		return err
	}
	defer func() {
		_ = imageFile.Close()
	}()
	if err = importImage(self.dockerSdk, imageName, imageFile); err != nil {
		if resetImage {
			imageLock.Unlock()
		}
		return err
	}
	return nil
}

// Close 外部调用（或看门人调用）的销毁入口
func (self *Plugin) Close() error {
	if self == nil || self.dockerSdk == nil || self.dockerSdk.Client == nil {
		return errors.New("docker client is required for plugin close")
	}
	if self.containerName == "" {
		return errors.New("plugin container name is required")
	}
	mutex := storage.NewMutex(fmt.Sprintf(storage.CacheKeyPluginLifecycleLock, self.dockerSdk.Name, self.containerName))
	mutex.Lock()
	defer mutex.Unlock()

	return self.close(false)
}

func (self *Plugin) close(removeImage bool) error {
	self.mu.Lock()
	defer self.mu.Unlock()

	containerTarget := self.containerID
	if containerTarget == "" {
		containerTarget = self.containerName
	}
	containerInfo, err := self.dockerSdk.Client.ContainerInspect(self.dockerSdk.Ctx, containerTarget)
	if errdefs.IsNotFound(err) {
		self.containerID = ""
		return nil
	}
	if err != nil {
		return err
	}
	if containerInfo.State != nil && containerInfo.State.Running {
		if err = self.dockerSdk.Client.ContainerStop(self.dockerSdk.Ctx, containerInfo.ID, container.StopOptions{}); err != nil && !errdefs.IsNotFound(err) {
			return err
		}
	}
	if err = self.dockerSdk.Client.ContainerRemove(self.dockerSdk.Ctx, containerInfo.ID, container.RemoveOptions{Force: true}); err != nil && !errdefs.IsNotFound(err) {
		return err
	}
	if errdefs.IsNotFound(err) {
		self.containerID = ""
		return nil
	}

	function.Wait(self.dockerSdk.Ctx, containerInfo.ID, func(v string) bool {
		if _, inspectErr := self.dockerSdk.Client.ContainerInspect(self.dockerSdk.Ctx, v); inspectErr != nil {
			slog.Debug("plugin delete", "name", self.Name, "id", containerInfo.Name)
			return true
		}
		return false
	})

	if removeImage && containerInfo.Config != nil {
		if err = self.dockerSdk.ImageRemove(self.dockerSdk.Ctx, filters.NewArgs(
			filters.Arg(docker.ImageFilterReference, containerInfo.Config.Image),
		)); err != nil {
			slog.Debug("plugin delete image", "name", self.Name, "id", containerInfo.Config.Image)
		}
	}
	self.containerID = ""
	return nil
}

func (self *Plugin) Exists() bool {
	self.mu.Lock()
	defer self.mu.Unlock()

	containerTarget := self.containerID
	if containerTarget == "" {
		containerTarget = self.containerName
	}
	if info, err := self.dockerSdk.Client.ContainerInspect(self.dockerSdk.Ctx, containerTarget); err == nil {
		return info.State.Running
	}
	return false
}

func (self *Plugin) ContainerName() string {
	self.mu.Lock()
	defer self.mu.Unlock()

	return self.containerName
}

func importImage(sdk *docker.Client, imageName string, imageFile fs.File) error {
	if err := sdk.ImageLoadFsFile(sdk.Ctx, imageFile); err != nil {
		return fmt.Errorf("load plugin image %s: %w", imageName, err)
	}
	return nil
}
