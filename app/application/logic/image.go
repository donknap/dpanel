package logic

import (
	"log/slog"
	"strings"

	dockerRegistry "github.com/docker/docker/api/types/registry"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	dockerTypes "github.com/donknap/dpanel/common/service/docker/types"
)

type Image struct{}

// GetRegistryConfig 根据镜像中的仓库地址解析实际源仓库配置。
// 精确匹配源仓库时直接使用其权限和代理；匹配到某个源仓库的代理时复用该源仓库配置，
// 并将镜像原地址放在代理列表首位；完全无法匹配时保留镜像原地址且不附加权限或代理。
func (self Image) GetRegistryConfig(registryUrl string) *dockerTypes.Registry {
	registryHost := registryUrl
	if registryAddress, err := function.ParseURL(registryUrl); err == nil {
		registryHost = registryAddress.Host
	}
	result := &dockerTypes.Registry{
		Address:      registryUrl,
		ProxyAddress: make([]string, 0),
	}

	var registryRow *entity.Registry
	var proxyRegistryRow *entity.Registry
	registryRows, err := dao.Registry.Order(dao.Registry.ID.Asc()).Find()
	if err != nil {
		slog.Warn("get registry config", "error", err)
		return result
	}
	for _, item := range registryRows {
		if item == nil || item.Setting == nil {
			continue
		}
		sourceHost := item.ServerAddress
		if sourceAddress, err := function.ParseURL(item.ServerAddress); err == nil {
			sourceHost = sourceAddress.Host
		}
		if strings.EqualFold(registryHost, sourceHost) {
			registryRow = item
			break
		}
		if proxyRegistryRow != nil {
			continue
		}
		for _, proxyAddress := range item.Setting.Proxy {
			if proxyURL, err := function.ParseURL(proxyAddress); err == nil && strings.EqualFold(registryHost, proxyURL.Host) {
				proxyRegistryRow = item
				break
			}
		}
	}

	firstProxyAddress := ""
	if registryRow == nil && proxyRegistryRow != nil {
		registryRow = proxyRegistryRow
		// 镜像本身使用的是代理地址时，该地址必须优先于同一源仓库配置的其它代理。
		firstProxyAddress = registryUrl
	}
	if registryRow == nil {
		return result
	}
	result.Address = registryRow.ServerAddress
	result.EnableHttp = registryRow.Setting.EnableHttp

	sourceHost := registryRow.ServerAddress
	if sourceAddress, err := function.ParseURL(registryRow.ServerAddress); err == nil {
		sourceHost = sourceAddress.Host
	}
	proxyAddresses := make([]string, 0, len(registryRow.Setting.Proxy)+1)
	if firstProxyAddress != "" {
		proxyAddresses = append(proxyAddresses, firstProxyAddress)
	}
	for _, proxyAddress := range registryRow.Setting.Proxy {
		proxyHost := ""
		proxyParsed := false
		if proxyURL, err := function.ParseURL(proxyAddress); err == nil {
			proxyHost = proxyURL.Host
			proxyParsed = true
		}
		if proxyParsed && strings.EqualFold(sourceHost, proxyHost) {
			continue
		}
		duplicate := false
		for _, existingAddress := range proxyAddresses {
			if proxyParsed {
				if existingURL, err := function.ParseURL(existingAddress); err == nil && strings.EqualFold(existingURL.Host, proxyHost) {
					duplicate = true
					break
				}
			}
			if existingAddress == proxyAddress {
				duplicate = true
				break
			}
		}
		if !duplicate {
			proxyAddresses = append(proxyAddresses, proxyAddress)
		}
	}
	result.ProxyAddress = proxyAddresses

	if username, password, ok := registryRow.Setting.Auth(); ok {
		result.Auth, err = dockerRegistry.EncodeAuthConfig(dockerRegistry.AuthConfig{
			Username:      username,
			Password:      password,
			ServerAddress: registryRow.ServerAddress,
		})
		if err != nil {
			slog.Debug("get registry auth string", "error", err)
			result.Auth = ""
		}
	}
	return result
}
