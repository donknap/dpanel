package logic

import (
	"github.com/docker/docker/api/types/registry"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/function"
	registry2 "github.com/donknap/dpanel/common/service/registry"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
	"log/slog"
	"strings"
)

type Image struct {
}

type ImageNameOption struct {
	Registry  string
	Name      string
	Version   string
	Namespace string
}

func (self Image) GetImageName(option *ImageNameOption) (imageName string) {
	if option.Name == "" {
		return ""
	}
	temp := registry2.GetImageTagDetail(option.Name)
	imageName = temp.ImageName

	if option.Version != "" {
		imageName = strings.Replace(imageName, temp.Version, option.Version, 1)
	}

	if option.Namespace != "" {
		if strings.Contains(imageName, "/") {
			imageName = strings.Replace(imageName, temp.Namespace, option.Namespace, 1)
		} else {
			imageName = option.Namespace + "/" + imageName
		}
	}

	if option.Registry != "" {
		imageName = option.Registry + "/" + imageName
	}

	return imageName
}

func (self Image) GetRegistryAuthString(serverAddress string, username string, password string) string {
	if password == "" || username == "" {
		return ""
	}
	if exists, u, p := self.GetRegistryAuth(username, password); exists {
		authString, err := registry.EncodeAuthConfig(registry.AuthConfig{
			Username: u,
			Password: p,
		})
		if err != nil {
			slog.Debug("get registry auth string", err.Error())
			return ""
		}
		return authString
	}
	return ""
}

func (self Image) GetRegistryAuth(username string, password string) (exists bool, u string, p string) {
	if password == "" || username == "" {
		return false, "", ""
	}
	password, _ = function.AseDecode(facade.GetConfig().GetString("app.name"), password)
	return true, username, password
}

func (self Image) GetRegistryList(imageName string) (proxyList []string, existsAuth bool, username string, password string) {
	proxyList = make([]string, 0)
	username = ""
	password = ""
	existsAuth = false

	tagDetail := registry2.GetImageTagDetail(imageName)
	// 从官方仓库拉取镜像不用权限
	registryList, _ := dao.Registry.Where(dao.Registry.ServerAddress.Eq(tagDetail.Registry)).Find()
	if registryList != nil && len(registryList) > 0 {
		if registryList[0] != nil && registryList[0].Setting.Password != "" {
			existsAuth, username, password = self.GetRegistryAuth(registryList[0].Setting.Username, registryList[0].Setting.Password)
		}
	}

	for _, registryRow := range registryList {
		if registryRow.Setting.Username == tagDetail.Namespace {
			existsAuth, username, password = self.GetRegistryAuth(registryRow.Setting.Username, registryRow.Setting.Password)
		}
		proxyList = append(proxyList, registryRow.Setting.Proxy...)
	}

	if len(proxyList) == 0 {
		proxyList = []string{
			tagDetail.Registry,
		}
	}

	return proxyList, existsAuth, username, password
}
