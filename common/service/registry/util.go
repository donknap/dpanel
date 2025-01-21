package registry

import (
	"fmt"
	"strings"
)

func GetImageTagDetail(tag string) *ImageTagDetail {
	tag = strings.TrimPrefix(strings.TrimPrefix(tag, "http://"), "https://")

	result := &ImageTagDetail{}
	if !strings.Contains(tag, ":") {
		tag += ":latest"
	}

	// 如果没有指定仓库地址，则默认为 docker.io
	temp := strings.Split(tag, "/")
	if !strings.Contains(temp[0], ".") || len(temp) == 1 {
		tag = DefaultRegistryDomain + "/" + tag
	}
	temp = strings.Split(tag, "/")
	result.Registry = temp[0]

	name := strings.Split(temp[len(temp)-1], ":")
	result.ImageName, result.Version = name[0], name[1]

	if len(temp) <= 2 {
		result.Namespace = "library"
	} else {
		result.Namespace = strings.Join(temp[1:len(temp)-1], "/")
	}
	result.BaseName = fmt.Sprintf("%s/%s", result.Namespace, result.ImageName)
	return result
}
