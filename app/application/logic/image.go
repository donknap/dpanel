package logic

import (
	"github.com/docker/docker/api/types/registry"
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
	temp := self.GetImageTagDetail(option.Name)
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

func (self Image) GetImageTagDetail(tag string) *registry2.ImageTagDetail {
	return registry2.GetImageTagDetail(tag)
}

func (self Image) GetRegistryAuthString(serverAddress string, username string, password string) string {
	if password == "" || username == "" {
		return ""
	}
	password, _ = function.AseDecode(facade.GetConfig().GetString("app.name"), password)
	authString, err := registry.EncodeAuthConfig(registry.AuthConfig{
		Username: username,
		Password: password,
	})
	if err != nil {
		slog.Debug("get registry auth string", err.Error())
		return ""
	}
	return authString
}

func (self Image) GetRegistryAuth(username string, password string) (exists bool, u string, p string) {
	if password == "" || username == "" {
		return false, "", ""
	}
	password, _ = function.AseDecode(facade.GetConfig().GetString("app.name"), password)
	return true, username, password
}
