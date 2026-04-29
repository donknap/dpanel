package plugin

import (
	"bytes"
	"embed"
	"html/template"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"strings"
	"sync"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/compose"
	"github.com/donknap/dpanel/common/service/docker"
	builder "github.com/donknap/dpanel/common/service/docker/container"
	"github.com/donknap/dpanel/common/service/docker/types"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/we7coreteam/registry-go-sdk"
)

const ExplorerName = "dpanel-plugin-explorer"

type CreateOption struct {
	RandomProxyContainerName bool               `json:"-"`
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
		containerName: name, // 默认与任务名称保持一致
	}

	option.Hash = function.Sha256Struct(option)

	var asset embed.FS
	if v, ok := storage.Cache.Get(storage.CacheKeyAsset); ok {
		asset = v.(embed.FS)
	} else {
		return nil, define.ErrorAssetEmpty
	}

	yamlTpl, err := asset.ReadFile("asset/plugin/" + name + "/compose.yaml")
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
	err = parser.Execute(buffer, function.StructToMap(option))
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

	if option.RandomProxyContainerName {
		p.containerName = ""
	} else if service.ContainerName != "" {
		p.containerName = service.ContainerName
	} else {
		p.containerName = name
	}

	dockerVersion, _ := dockerSdk.Client.ServerVersion(dockerSdk.Ctx)
	if imageTarUrl, ok := serviceExt.ImageTar[dockerVersion.Arch]; ok {
		if imageTarUrl != "" && strings.HasPrefix(imageTarUrl, "asset/plugin") {
			imageFile, err := asset.Open(imageTarUrl)
			if os.IsNotExist(err) {
				return nil, err
			}
			defer func() {
				_ = imageFile.Close()
			}()
			err = importImage(dockerSdk, service.Image, imageFile)
			if err != nil {
				return nil, err
			}
		}
	}

	if serviceExt.ImageProxy != nil {
		imageNameDetail := function.ImageTag(service.Image)

		for _, proxy := range serviceExt.ImageProxy {
			reg := registry.New(
				registry.WithAddress(proxy),
			)
			if ok, _, manifestErr := reg.Client().ManifestExist(imageNameDetail.BaseName, imageNameDetail.Version); manifestErr == nil && ok {
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

	}

	return p, nil
}

type Plugin struct {
	Name          string
	containerName string
	mu            sync.Mutex
	dockerSdk     *docker.Client
	composeTask   *compose.Task
}

func (self *Plugin) Create() error {
	service, serviceExt, err := self.composeTask.GetService(self.Name)
	if err != nil {
		return err
	}

	containerInfo, err := self.dockerSdk.Client.ContainerInspect(self.dockerSdk.Ctx, service.ContainerName)
	if err == nil {
		slog.Info("plugin", "create-explorer", containerInfo.ID)
		if containerInfo.State.Restarting {
			goto recreate
		}
		if !containerInfo.State.Running {
			if err = self.dockerSdk.Client.ContainerStart(self.dockerSdk.Ctx, containerInfo.ID, container.StartOptions{}); err == nil {
				return nil
			}
			goto recreate
		}
		if v, ok := containerInfo.Config.Labels[define.DPanelLabelContainerHash]; ok && v == service.Labels[define.DPanelLabelContainerHash] {
			return nil
		}
	}

recreate:
	err = self.Close()
	if err != nil {
		return err
	}

	self.mu.Lock()
	defer self.mu.Unlock()

	options := []builder.Option{
		builder.WithImage(service.Image, false),
		builder.WithContainerName(self.containerName),
		builder.WithLabel(function.PluckMapWalkArray(service.Labels, func(key string, value string) (types.ValueItem, bool) {
			return types.ValueItem{
				Name:  key,
				Value: value,
			}, true
		})...),
		builder.WithPrivileged(service.Privileged),
		builder.WithRestartPolicy(&types.RestartPolicy{
			Name: service.Restart,
		}),
		builder.WithPid(service.Pid),
		builder.WithVolumesFromContainerName(serviceExt.External.VolumesFrom...),
		builder.WithCommand(service.Command),
		builder.WithWorkDir(service.WorkingDir),
	}

	for _, item := range service.Volumes {
		options = append(options, builder.WithVolume(types.VolumeItem{
			Host:       item.Source,
			Dest:       item.Target,
			Permission: "write",
		}))
	}

	for _, item := range serviceExt.External.Volumes {
		path := strings.Split(item, ":")
		options = append(options, builder.WithVolume(types.VolumeItem{
			Host:       path[0],
			Dest:       path[1],
			Permission: "write",
		}))
	}

	b, err := builder.New(options...)
	if err != nil {
		return err
	}
	response, err := b.Execute()
	if err != nil {
		return err
	}

	err = self.dockerSdk.Client.ContainerStart(self.dockerSdk.Ctx, response.ID, container.StartOptions{})
	if err != nil {
		return err
	}

	self.containerName = response.ID
	function.Wait(self.dockerSdk.Ctx, response.ID, func(v string) bool {
		if info, err := self.dockerSdk.Client.ContainerInspect(self.dockerSdk.Ctx, v); err == nil && info.State.Running {
			return true
		}
		return false
	})

	return nil
}

// Close 外部调用（或看门人调用）的销毁入口
func (self *Plugin) Close() error {
	self.mu.Lock()
	defer self.mu.Unlock()

	_, serviceExt, err := self.composeTask.GetService(self.Name)
	if err != nil {
		return err
	}

	if containerInfo, err := self.dockerSdk.Client.ContainerInspect(self.dockerSdk.Ctx, self.containerName); err == nil {
		if containerInfo.State.Running {
			if err = self.dockerSdk.Client.ContainerStop(self.dockerSdk.Ctx, containerInfo.ID, container.StopOptions{}); err != nil {
				return err
			}
		}
		if err = self.dockerSdk.Client.ContainerRemove(self.dockerSdk.Ctx, containerInfo.ID, container.RemoveOptions{
			Force: true,
		}); err != nil {
			return err
		}

		function.Wait(self.dockerSdk.Ctx, containerInfo.ID, func(v string) bool {
			if _, err = self.dockerSdk.Client.ContainerInspect(self.dockerSdk.Ctx, v); err != nil {
				slog.Debug("plugin delete explorer", "id", containerInfo.Name)
				return true
			}
			return false
		})

		if serviceExt.ImageAutoRemove {
			if err = self.dockerSdk.ImageRemoveAll(self.dockerSdk.Ctx, containerInfo.Config.Image); err != nil {
				slog.Debug("plugin delete explorer image", "id", containerInfo.Config.Image)
			}
		}
	}
	return nil
}

func (self *Plugin) Exists() bool {
	self.mu.Lock()
	defer self.mu.Unlock()

	if info, err := self.dockerSdk.Client.ContainerInspect(self.dockerSdk.Ctx, self.Name); err == nil {
		return info.State.Running
	}
	return false
}

func importImage(sdk *docker.Client, imageName string, imageFile fs.File) error {
	if _, err := sdk.Client.ImageInspect(sdk.Ctx, imageName); err != nil {
		if _, err = docker.Sdk.Client.ImageRemove(sdk.Ctx, imageName, image.RemoveOptions{
			Force:         true,
			PruneChildren: true,
		}); err != nil {
			slog.Warn("plugin create explorer", "delete old image", imageName, "error", err)
		}
		err = sdk.ImageLoadFsFile(sdk.Ctx, imageFile)
		if err != nil {
			return err
		}
	}
	return nil
}
