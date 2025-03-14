package plugin

import (
	"bytes"
	"embed"
	"errors"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/donknap/dpanel/common/service/compose"
	"github.com/donknap/dpanel/common/service/docker"
	builder "github.com/donknap/dpanel/common/service/docker/container"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
	"html/template"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"
)

const PluginExplorer = "explorer"
const PluginBackup = "backup"
const PluginWebShell = "webshell"

const (
	LabelContainerAutoRemove = "com.dpanel.container.auto_remove"
	LabelContainerTitle      = "com.dpanel.container.title"
)

type TemplateParser struct {
	Volumes       []string
	Command       []string
	ContainerName string
	compose.ExtService
}

func NewPlugin(name string, composeData map[string]*TemplateParser) (*plugin, error) {
	var asset embed.FS
	err := facade.GetContainer().NamedResolve(&asset, "asset")
	if err != nil {
		return nil, err
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
	err = parser.Execute(buffer, composeData)
	if err != nil {
		return nil, err
	}
	composer, err := compose.NewCompose(compose.WithYamlContent(buffer.String()))
	if err != nil {
		return nil, err
	}
	_ = os.RemoveAll(composer.Project.WorkingDir)
	obj := &plugin{
		asset:   asset,
		name:    name,
		compose: composer,
	}
	return obj, nil
}

type plugin struct {
	asset   embed.FS
	name    string
	compose *compose.Wrapper
}

func (self plugin) Create() (string, error) {
	service, serviceExt, err := self.compose.GetService(self.name)
	if err != nil {
		return "", err
	}
	pluginContainerInfo, err := docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, service.ContainerName)
	if err == nil {
		// 如果容器在，并且有 auto-remove 参数，则删除掉
		if serviceExt.AutoRemove {
			err = self.Destroy()
			if err != nil {
				return "", err
			}
		} else {
			slog.Debug("plugin", "create-explorer", pluginContainerInfo.ID)
			if !pluginContainerInfo.State.Running {
				err = docker.Sdk.Client.ContainerStart(docker.Sdk.Ctx, pluginContainerInfo.ID, container.StartOptions{})
				if err != nil {
					return "", err
				}
			}
			return service.ContainerName, nil
		}
	}
	dockerVersion, _ := docker.Sdk.Client.ServerVersion(docker.Sdk.Ctx)

	imageUrl := service.Image
	imageTryPull := true

	if imageTarUrl, ok := serviceExt.ImageTar[dockerVersion.Arch]; ok {
		if imageTarUrl != "" && strings.HasPrefix(imageTarUrl, "asset/plugin") {
			imageTryPull = false
			err = self.importImage(service.Image, imageTarUrl)
			if err != nil {
				return "", err
			}
		}
	}

	options := []builder.Option{
		builder.WithImage(imageUrl, imageTryPull),
		builder.WithContainerName(service.ContainerName),
		builder.WithLabel(docker.NewValueItemFromMap(service.Labels)...),
		builder.WithPrivileged(service.Privileged),
		builder.WithAutoRemove(serviceExt.AutoRemove),
		builder.WithRestartPolicy(service.Restart),
		builder.WithPid(service.Pid),
		builder.WithVolumesFromContainerName(serviceExt.External.VolumesFrom...),
		builder.WithCommand(service.Command),
	}

	for _, item := range service.Volumes {
		options = append(options, builder.WithVolume(docker.VolumeItem{
			Host:       item.Source,
			Dest:       item.Target,
			Permission: "write",
		}))
	}
	for _, item := range serviceExt.External.Volumes {
		path := strings.Split(item, ":")
		options = append(options, builder.WithVolume(docker.VolumeItem{
			Host:       path[0],
			Dest:       path[1],
			Permission: "write",
		}))
	}
	b, err := builder.New(options...)
	if err != nil {
		return "", err
	}
	response, err := b.Execute()
	if err != nil {
		return "", err
	}
	err = docker.Sdk.Client.ContainerStart(docker.Sdk.Ctx, response.ID, container.StartOptions{})
	if err != nil {
		return "", err
	}
	for {
		if _, err := docker.Sdk.ContainerInfo(response.ID); err == nil {
			return service.ContainerName, nil
		} else {
			time.Sleep(time.Second)
		}
	}
}

func (self plugin) Destroy() error {
	service, _, err := self.compose.GetService(self.name)
	if err != nil {
		return err
	}
	containerRow, err := docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, service.ContainerName)
	if err == nil {
		if v, ok := containerRow.Config.Labels[LabelContainerAutoRemove]; ok && v == "false" {
			return nil
		}
		if containerRow.State.Running {
			err = docker.Sdk.Client.ContainerStop(docker.Sdk.Ctx, service.ContainerName, container.StopOptions{})
			if err != nil {
				return err
			}
		}
		err = docker.Sdk.Client.ContainerRemove(docker.Sdk.Ctx, service.ContainerName, container.RemoveOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func (self plugin) importImage(imageName string, imagePath string) error {
	_, err := docker.Sdk.Client.ImageInspect(docker.Sdk.Ctx, imageName)
	if err == nil {
		_, err = docker.Sdk.Client.ImageRemove(docker.Sdk.Ctx, imageName, image.RemoveOptions{
			Force:         true,
			PruneChildren: true,
		})
		slog.Debug("plugin", "create-explorer", "delete old image", imageName)
		if err != nil {
			return err
		}
	}
	imageFile, err := self.asset.Open(imagePath)
	if os.IsNotExist(err) {
		return errors.New("插件暂不支持该平台，请提交 issues ")
	}
	reader, err := docker.Sdk.Client.ImageLoad(docker.Sdk.Ctx, imageFile, client.ImageLoadWithQuiet(false))
	if err != nil {
		return err
	}
	_, err = io.Copy(io.Discard, reader.Body)
	if err != nil {
		return err
	}
	return nil
}
