package registry

import (
	"fmt"
	"net/url"
	"strings"
)

func GetImageTagDetail(tag string) *ImageTagDetail {
	tag = strings.TrimPrefix(strings.TrimPrefix(tag, "http://"), "https://")
	result := &ImageTagDetail{}

	// 如果没有指定仓库地址，则默认为 docker.io
	noRegistryUrl := false
	temp := strings.Split(tag, "/")
	if !strings.Contains(temp[0], ".") || len(temp) == 1 {
		noRegistryUrl = true
		tag = DefaultRegistryDomain + "/" + tag
	}
	temp = strings.Split(tag, "/")
	// 先补齐 registry 地址后再判断是否有标签，仓库地址中可能包含端口号
	if !strings.Contains(strings.Join(temp[1:], "/"), ":") {
		tag += ":latest"
	}
	temp = strings.Split(tag, "/")
	result.Registry = temp[0]

	name := strings.Split(temp[len(temp)-1], ":")
	result.ImageName, result.Version = name[0], strings.Join(name[1:], ":")

	// 兼容使用 digest 标识版本号的情况
	if strings.Contains(result.Version, "@") {
		//result.Version = strings.Split(result.Version, "@")[1]
	}

	if len(temp) <= 2 {
		if noRegistryUrl {
			result.Namespace = "library"
		}
	} else {
		result.Namespace = strings.Join(temp[1:len(temp)-1], "/")
	}
	if result.Namespace != "" {
		result.BaseName = fmt.Sprintf("%s/%s", result.Namespace, result.ImageName)
	} else {
		result.BaseName = result.ImageName
	}

	return result
}

func GetRegistryUrl(address string) url.URL {
	host := strings.TrimSuffix(address, "/")
	protocol := "https"

	parseUrl, err := url.Parse(host)
	if err == nil && parseUrl.Host != "" {
		host = parseUrl.Host
		protocol = parseUrl.Scheme
	}

	return url.URL{
		Scheme: protocol,
		Host:   host,
		Path:   "/v2/",
	}
}
