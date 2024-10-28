package logic

import (
	"errors"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/entity"
	"golang.org/x/exp/maps"
)

type DockerEnv struct {
}

func (self DockerEnv) UpdateEnv(data *accessor.DockerClientResult) {
	setting, err := Setting{}.GetValue(SettingGroupSetting, SettingGroupSettingDocker)
	if err != nil || setting.Value == nil || setting.Value.Docker == nil {
		setting = &entity.Setting{
			GroupName: SettingGroupSetting,
			Name:      SettingGroupSettingDocker,
			Value: &accessor.SettingValueOption{
				Docker: make(map[string]*accessor.DockerClientResult, 0),
			},
		}
	}
	dockerList := map[string]*accessor.DockerClientResult{
		data.Name: data,
	}
	maps.Copy(setting.Value.Docker, dockerList)
	_ = Setting{}.Save(setting)
	return
}

func (self DockerEnv) GetEnvByName(name string) (*accessor.DockerClientResult, error) {
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
