package logic

import (
	"errors"
	"os"

	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker/types"
	"github.com/donknap/dpanel/common/types/define"
	"golang.org/x/exp/maps"
)

type Env struct {
}

func (self Env) UpdateEnv(data *types.DockerEnv) {
	setting, err := Setting{}.GetValue(SettingGroupSetting, SettingGroupSettingDocker)
	if err != nil || setting.Value == nil || setting.Value.Docker == nil {
		setting = &entity.Setting{
			GroupName: SettingGroupSetting,
			Name:      SettingGroupSettingDocker,
			Value: &accessor.SettingValueOption{
				Docker: make(map[string]*types.DockerEnv, 0),
			},
		}
	}
	dockerList := map[string]*types.DockerEnv{
		data.Name: data,
	}
	maps.Copy(setting.Value.Docker, dockerList)
	_ = Setting{}.Save(setting)
	return
}

func (self Env) GetEnvByName(name string) (*types.DockerEnv, error) {
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

func (self Env) GetDefaultEnv() (*types.DockerEnv, error) {
	dockerEnvList := make(map[string]*types.DockerEnv)
	Setting{}.GetByKey(SettingGroupSetting, SettingGroupSettingDocker, &dockerEnvList)
	if v, ok := dockerEnvList[os.Getenv("DP_DEFAULT_DOCKER_ENV")]; ok {
		return v, nil
	}

	if v := function.PluckMapWalkArray(dockerEnvList, func(k string, v *types.DockerEnv) (*types.DockerEnv, bool) {
		if v.Default {
			return v, true
		}
		return nil, false
	}); !function.IsEmptyArray(v) {
		return v[0], nil
	}

	if v := function.PluckMapWalkArray(dockerEnvList, func(k string, v *types.DockerEnv) (*types.DockerEnv, bool) {
		if v.Name == define.DockerDefaultClientName {
			return v, true
		}
		return nil, false
	}); !function.IsEmptyArray(v) {
		return v[0], nil
	}

	return nil, errors.New("default docker env does not exist")
}
