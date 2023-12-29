package plugin

import (
	"embed"
	"encoding/json"
	"github.com/docker/docker/api/types"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/we7coreteam/w7-rangine-go-support/src/facade"
	"io"
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
	if self.setting.Image != "" {
		err := self.importImage()
		if err != nil {
			return "", err
		}
		err = self.runContainer()
		if err != nil {
			return "", err
		}
	}
	return self.setting.Name, nil
}

func (self plugin) importImage() error {
	_, _, err := docker.Sdk.Client.ImageInspectWithRaw(docker.Sdk.Ctx, self.setting.ImageName)
	if err == nil {
		return nil
	}
	imageFile, _ := self.asset.Open(self.setting.Image)
	reader, _ := docker.Sdk.Client.ImageLoad(docker.Sdk.Ctx, imageFile, false)
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
	builder.WithImage(self.setting.ImageName, false)
	builder.WithAutoRemove()
	builder.WithContainerName(self.setting.Name)
	if self.setting.Env.PidHost {
		builder.WithPid("host")
	}
	response, err := builder.Execute()
	if err != nil {
		return err
	}
	err = docker.Sdk.Client.ContainerStart(docker.Sdk.Ctx, response.ID, types.ContainerStartOptions{})
	if err != nil {
		return err
	}
	return nil
}
