package logic

import (
	"fmt"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	registry2 "github.com/donknap/dpanel/common/service/registry"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
	"log/slog"
)

type Image struct {
}

func (self Image) GetRegistryConfig(imageName string) registry2.Config {
	result := registry2.Config{
		Proxy: make([]string, 0),
	}
	imageNameDetail := registry2.GetImageTagDetail(imageName)

	if docker.Sdk.Client != nil && imageNameDetail.Registry == registry2.DefaultRegistryDomain {
		// 获取docker 配置中的加速地址
		// 暂时注释掉，直接取 daemon.json 的镜像地址问题太多
		//if dockerInfo, err := docker.Sdk.Client.Info(docker.Sdk.Ctx); err == nil && dockerInfo.RegistryConfig != nil && !function.IsEmptyArray(dockerInfo.RegistryConfig.Mirrors) {
		//	result.Proxy = dockerInfo.RegistryConfig.Mirrors
		//}
	}

	registryList, _ := dao.Registry.Where(dao.Registry.ServerAddress.Eq(imageNameDetail.Registry)).Find()
	for _, item := range registryList {
		if item.Setting == nil {
			continue
		}
		if item.Setting.EnableHttp {
			result.Proxy = append(result.Proxy, fmt.Sprintf("http://"+item.ServerAddress))
		}
		if !function.IsEmptyArray(item.Setting.Proxy) {
			result.Proxy = append(result.Proxy, item.Setting.Proxy...)
		}
		if item.Setting.Username != "" && item.Setting.Password != "" {
			result.Username = item.Setting.Username
			password, err := function.AseDecode(facade.GetConfig().GetString("app.name"), item.Setting.Password)
			if err != nil {
				slog.Debug("image registry decode password", "error", err)
			}
			result.Password = password
			result.ExistsAuth = true
		}
	}

	if !function.InArray(result.Proxy, imageNameDetail.Registry) {
		result.Proxy = append(result.Proxy, imageNameDetail.Registry)
	}
	return result
}
