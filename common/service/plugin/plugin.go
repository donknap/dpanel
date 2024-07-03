package plugin

import (
	"embed"
	"encoding/json"
	"errors"
	"github.com/docker/docker/api/types/container"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/we7coreteam/w7-rangine-go-support/src/facade"
	"io"
	"os"
	"strings"
)

func NewPlugin(name string) (*plugin, error) {
	var asset embed.FS
	err := facade.GetContainer().NamedResolve(&asset, "asset")
	if err != nil {
		return nil, err
	}
	pluginSettingStr, err := asset.ReadFile("asset/plugin/" + name + "/setting.json")
	if err != nil {
		return nil, err
	}
	var ps pluginSetting
	json.Unmarshal(pluginSettingStr, &ps)
	obj := &plugin{
		asset:   asset,
		setting: &ps,
	}
	return obj, nil
}

type plugin struct {
	asset   embed.FS
	setting *pluginSetting
}

func (self plugin) Create() (string, error) {
	dockerVersion, _ := docker.Sdk.Client.ServerVersion(docker.Sdk.Ctx)
	if image, ok := self.setting.Image[dockerVersion.Arch]; ok {
		if image != "" {
			if strings.HasPrefix(image, "asset/plugin") {
				err := self.importImage(image)
				if err != nil {
					return "", err
				}
			} else {
				self.setting.ImageName = image
			}
			err := self.runContainer()
			if err != nil {
				return "", err
			}
		}
		if self.setting.Container.Init != "" {
			_, err := Command{}.Result(self.setting.Name, self.setting.Container.Init)
			if err != nil {
				return "", err
			}
		}
		return self.setting.Name, nil
	} else {
		return "", errors.New("插件暂不支持该平台，请提交 issues ")
	}
}

func (self plugin) importImage(imagePath string) error {
	_, _, err := docker.Sdk.Client.ImageInspectWithRaw(docker.Sdk.Ctx, self.setting.ImageName)
	if err == nil {
		return nil
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

func (self plugin) runContainer() error {
	_, err := docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, self.setting.Name)
	if err == nil {
		return nil
	}
	builder := docker.Sdk.GetContainerCreateBuilder()
	if self.setting.Env.Privileged {
		builder.WithPrivileged()
	}
	builder.WithImage(self.setting.ImageName, true)
	builder.WithAutoRemove()
	builder.WithContainerName(self.setting.Name)
	if self.setting.Env.PidHost {
		builder.WithPid("host")
	}
	response, err := builder.Execute()
	if err != nil {
		return err
	}
	err = docker.Sdk.Client.ContainerStart(docker.Sdk.Ctx, response.ID, container.StartOptions{})
	if err != nil {
		return err
	}
	return nil
}
