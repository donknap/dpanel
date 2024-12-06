package logic

import (
	"github.com/docker/docker/api/types/registry"
	"github.com/donknap/dpanel/common/function"
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

type imageTagDetail struct {
	Registry  string
	Namespace string
	ImageName string
	Version   string
	Tag       string
}

func (self Image) GetImageTagDetail(tag string) *imageTagDetail {
	tag = strings.TrimPrefix(strings.TrimPrefix(tag, "http://"), "https://")
	result := &imageTagDetail{}
	if !strings.Contains(tag, ":") {
		tag += ":latest"
	}
	result.Tag = tag
	// 如果没有指定仓库地址，则默认为 docker.io
	temp := strings.Split(tag, "/")
	if !strings.Contains(temp[0], ".") || len(temp) == 1 {
		tag = "docker.io/" + tag
	}

	temp = strings.Split(tag, "/")
	result.Registry = temp[0]
	result.ImageName = strings.Join(temp[1:], "/")

	if len(temp) <= 2 {
		temp = strings.Split(result.ImageName, ":")
		result.Namespace = temp[0]
		result.Version = temp[1]
	} else {
		result.Namespace = temp[1]
		temp = strings.Split(result.ImageName, ":")
		result.Version = temp[1]
	}
	return result
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
