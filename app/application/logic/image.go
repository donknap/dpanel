package logic

import (
	"fmt"
	"log/slog"

	"github.com/docker/docker/api/types/registry"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/we7coreteam/registry-go-sdk/types"
)

type Registry struct {
	config  registry.AuthConfig
	Address []string
}

// AuthString Docker pull/push 权限
func (self Registry) AuthString() string {
	authString, err := registry.EncodeAuthConfig(self.config)
	if err != nil {
		slog.Debug("get registry auth string", err.Error())
		return ""
	}
	return authString
}

// Credential registry 仓库权限
func (self Registry) Credential() types.Credential {
	return types.Credential{
		AccessKey:    self.config.Username,
		AccessSecret: self.config.Password,
	}
}

type Image struct {
}

func (self Image) GetRegistryConfig(registryUrl string) *Registry {
	if docker.Sdk.Client != nil && registryUrl == define.RegistryDefaultName {
		// 获取 docker 配置中的加速地址
		// 暂时注释掉，直接取 daemon.json 的镜像地址问题太多
		//if dockerInfo, err := docker.Sdk.Client.Info(docker.Sdk.Ctx); err == nil && dockerInfo.RegistryConfig != nil && !function.IsEmptyArray(dockerInfo.RegistryConfig.Mirrors) {
		//	result.Proxy = dockerInfo.RegistryConfig.Mirrors
		//}
	}
	result := &Registry{
		Address: make([]string, 0),
		config: registry.AuthConfig{
			ServerAddress: registryUrl,
		},
	}

	registryRow, err := dao.Registry.Where(dao.Registry.ServerAddress.Eq(registryUrl)).First()
	if err != nil || registryRow == nil || registryRow.Setting == nil {
		result.Address = append(result.Address, registryUrl)
		return result
	}

	if registryRow.Setting.EnableHttp {
		result.Address = append(result.Address, fmt.Sprintf("http://"+registryRow.ServerAddress))
	}

	if !function.IsEmptyArray(registryRow.Setting.Proxy) {
		result.Address = append(result.Address, registryRow.Setting.Proxy...)
	}

	if !function.InArray(result.Address, registryUrl) {
		result.Address = append(result.Address, registryUrl)
	}

	if registryRow.Setting != nil {
		if username, password, ok := registryRow.Setting.Auth(); ok {
			result.config.Username = username
			result.config.Password = password
		}
	}

	return result
}
