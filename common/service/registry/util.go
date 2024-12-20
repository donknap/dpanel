package registry

import "strings"

func GetImageTagDetail(tag string) *ImageTagDetail {
	tag = strings.TrimPrefix(strings.TrimPrefix(tag, "http://"), "https://")
	result := &ImageTagDetail{}
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
		result.Namespace = "library"
		result.ImageName = result.Namespace + "/" + result.ImageName
		result.Version = temp[1]
	} else {
		result.Namespace = temp[1]
		temp = strings.Split(result.ImageName, ":")
		result.Version = temp[1]
	}
	return result
}
