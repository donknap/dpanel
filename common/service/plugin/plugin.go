package plugin

import (
	"embed"
	"errors"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
	"html/template"
	"io"
	"log/slog"
	"os"
	"strings"
)

func NewPlugin(name string, composeData map[string]docker.ComposeService) (*plugin, error) {
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

	yamlResult := newResult()
	err = parser.Execute(yamlResult, composeData)
	if err != nil {
		return nil, err
	}
	compose, err := docker.NewYaml(string(yamlResult.GetData()))
	if err != nil {
		return nil, err
	}
	obj := &plugin{
		asset:   asset,
		name:    name,
		compose: compose,
	}
	return obj, nil
}

type plugin struct {
	asset   embed.FS
	name    string
	compose *docker.DockerComposeYamlV2
}

type CreateOption struct {
	VolumesForm string
	Volumes     []string
	Cmd         string
}

func (self plugin) Create() (string, error) {
	service, err := self.compose.GetService(self.name)
	if err != nil {
		return "", err
	}
	pluginContainerInfo, err := docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, service.ContainerName)
	if err == nil {
		// 如果容器在，并且有 auto-remove 参数，则删除掉
		if service.Extend.AutoRemove {
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

	if imageTarUrl, ok := service.Extend.ImageLocalTar[dockerVersion.Arch]; ok {
		if imageTarUrl != "" && strings.HasPrefix(imageTarUrl, "asset/plugin") {
			imageTryPull = false
			err = self.importImage(service.Image, imageTarUrl)
			if err != nil {
				return "", err
			}
		}
	}

	builder := docker.Sdk.GetContainerCreateBuilder()
	builder.WithImage(imageUrl, imageTryPull)
	builder.WithContainerName(service.ContainerName)

	if service.Privileged {
		builder.WithPrivileged()
	}
	if service.Extend.AutoRemove {
		builder.WithAutoRemove()
	}
	if service.Restart != "" {
		builder.WithRestart(service.Restart)
	}
	if service.Pid != "" {
		builder.WithPid(service.Pid)
	}
	for _, item := range service.VolumesFrom {
		builder.WithContainerVolume(item)
	}
	switch cmd := service.Command.(type) {
	case []string:
		if !function.IsEmptyArray(cmd) {
			builder.WithCommand(cmd)
		}
	case []interface{}:
		builder.WithCommand(function.ConvertArray[string](cmd))
	case string:
		builder.WithCommandStr(cmd)
	}

	for _, item := range service.Volumes {
		path := strings.Split(item, ":")
		builder.WithVolume(path[0], path[1], path[2])
	}
	response, err := builder.Execute()
	if err != nil {
		return "", err
	}
	err = docker.Sdk.Client.ContainerStart(docker.Sdk.Ctx, response.ID, container.StartOptions{})
	if err != nil {
		return "", err
	}
	return service.ContainerName, nil
}

func (self plugin) Destroy() error {
	service, err := self.compose.GetService(self.name)
	if err != nil {
		return err
	}
	_, err = docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, service.ContainerName)
	if err == nil {
		err = docker.Sdk.Client.ContainerStop(docker.Sdk.Ctx, service.ContainerName, container.StopOptions{})
		if err != nil {
			return err
		}
		err = docker.Sdk.Client.ContainerRemove(docker.Sdk.Ctx, service.ContainerName, container.RemoveOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func (self plugin) importImage(imageName string, imagePath string) error {
	_, _, err := docker.Sdk.Client.ImageInspectWithRaw(docker.Sdk.Ctx, imageName)
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
	reader, err := docker.Sdk.Client.ImageLoad(docker.Sdk.Ctx, imageFile, false)
	if err != nil {
		return err
	}
	_, err = io.Copy(io.Discard, reader.Body)
	if err != nil {
		return err
	}
	return nil
}
