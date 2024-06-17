package logic

import (
	"github.com/donknap/dpanel/common/function"
	"github.com/we7coreteam/w7-rangine-go-support/src/facade"
	"strings"
)

type Image struct {
}

type ImageNameOption struct {
	Registry string
	Name     string
	Version  string
}

func (self Image) GetImageName(option *ImageNameOption) (imageName string) {
	imageName = option.Name
	if option.Name == "" {
		return imageName
	}
	if strings.Contains(imageName, ":") {
		s := strings.Split(imageName, ":")
		if option.Version == "" {
			option.Version = s[1]
		}
		option.Name = s[0]
	}

	if option.Registry != "" {
		imageName = option.Registry + "/" + option.Name
	}
	if option.Version == "" {
		imageName += ":latest"
	} else {
		imageName += ":" + option.Version
	}
	return imageName
}

type imageTagDetail struct {
	Registry  string
	Namespace string
	ImageName string
}

func (self Image) GetImageTagDetail(tag string) *imageTagDetail {
	result := &imageTagDetail{}

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
	} else {
		result.Namespace = temp[1]
	}
	return result
}

func (self Image) GetRegistryAuthString(serverAddress string, username string, password string) string {
	password, _ = function.AseDecode(facade.GetConfig().GetString("app.name"), password)
	authString := function.Base64Encode(struct {
		Username      string `json:"username"`
		Password      string `json:"password"`
		ServerAddress string `json:"serveraddress"`
	}{
		Username:      username,
		Password:      password,
		ServerAddress: serverAddress,
	})
	return authString
}
