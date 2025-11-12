package logic

import (
	"fmt"

	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	registry2 "github.com/donknap/dpanel/common/service/docker/registry"
	"github.com/donknap/dpanel/common/types/define"
)

type Image struct {
}

func (self Image) GetRegistryConfig(registryUrl string) *registry2.Registry {
	if docker.Sdk.Client != nil && registryUrl == define.RegistryDefaultName {
		// 获取 docker 配置中的加速地址
		// 暂时注释掉，直接取 daemon.json 的镜像地址问题太多
		//if dockerInfo, err := docker.Sdk.Client.Info(docker.Sdk.Ctx); err == nil && dockerInfo.RegistryConfig != nil && !function.IsEmptyArray(dockerInfo.RegistryConfig.Mirrors) {
		//	result.Proxy = dockerInfo.RegistryConfig.Mirrors
		//}
	}
	registryRow, err := dao.Registry.Where(dao.Registry.ServerAddress.Eq(registryUrl)).First()
	if err != nil || registryRow == nil || registryRow.Setting == nil {
		return registry2.NewRegistry(
			registry2.WithHost(registryUrl),
			registry2.WithAddress(registryUrl),
		)
	}

	proxy := make([]string, 0)
	if registryRow.Setting.EnableHttp {
		proxy = append(proxy, fmt.Sprintf("http://"+registryRow.ServerAddress))
	}

	if !function.IsEmptyArray(registryRow.Setting.Proxy) {
		proxy = append(proxy, registryRow.Setting.Proxy...)
	}

	if !function.InArray(proxy, registryUrl) {
		proxy = append(proxy, registryUrl)
	}

	option := []registry2.Option{
		registry2.WithAddress(proxy...),
		registry2.WithHost(registryUrl),
	}

	if registryRow.Setting != nil {
		if username, password, ok := registryRow.Setting.Auth(); ok {
			option = append(option, registry2.WithBasicAuth(username, password))
		}
	}

	return registry2.NewRegistry(option...)
}
