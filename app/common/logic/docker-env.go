package logic

import (
	"errors"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"golang.org/x/exp/maps"
)

type DockerEnv struct {
}

func (self DockerEnv) UpdateEnv(data *docker.Client) {
	setting, err := Setting{}.GetValue(SettingGroupSetting, SettingGroupSettingDocker)
	if err != nil || setting.Value == nil || setting.Value.Docker == nil {
		setting = &entity.Setting{
			GroupName: SettingGroupSetting,
			Name:      SettingGroupSettingDocker,
			Value: &accessor.SettingValueOption{
				Docker: make(map[string]*docker.Client, 0),
			},
		}
	}
	dockerList := map[string]*docker.Client{
		data.Name: data,
	}
	maps.Copy(setting.Value.Docker, dockerList)
	_ = Setting{}.Save(setting)
	return
}

func (self DockerEnv) GetEnvByName(name string) (*docker.Client, error) {
	dockerEnvSetting, err := Setting{}.GetValue(SettingGroupSetting, SettingGroupSettingDocker)
	if err != nil {
		return nil, err
	}
	if dockerEnv, ok := dockerEnvSetting.Value.Docker[name]; ok {
		return dockerEnv, nil
	} else {
		return nil, errors.New("docker env not found")
	}
}

func (self DockerEnv) GetDefaultEnv() (*docker.Client, error) {
	dockerEnvList := make(map[string]*docker.Client)
	Setting{}.GetByKey(SettingGroupSetting, SettingGroupSettingDocker, &dockerEnvList)
	if v := function.PluckMapWalkArray(dockerEnvList, func(k string, v *docker.Client) (*docker.Client, bool) {
		if v.Default {
			return v, true
		}
		return nil, false
	}); !function.IsEmptyArray(v) {
		return v[0], nil
	}
	if v := function.PluckMapWalkArray(dockerEnvList, func(k string, v *docker.Client) (*docker.Client, bool) {
		if v.Name == docker.DefaultClientName {
			return v, true
		}
		return nil, false
	}); !function.IsEmptyArray(v) {
		return v[0], nil
	}
	return nil, errors.New("default docker env does not exist")
}
