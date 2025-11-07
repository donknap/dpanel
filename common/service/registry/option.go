package registry

import (
	"strings"
	"time"

	"github.com/donknap/dpanel/common/service/registry/types"
)

type Option func(*Registry)

func WithCredentials(credential types.Credential) Option {
	return func(registry *Registry) {
		registry.credential = credential
	}
}

func WithCredentialsToken(token string) Option {
	return func(registry *Registry) {
		if token != "" {
			registry.authToken = token
		}
	}
}

func WithRequestCacheTime(cacheTime time.Duration) Option {
	return func(registry *Registry) {
		registry.cacheTime = cacheTime
	}
}

func WithRegistryHost(host string) Option {
	return func(registry *Registry) {
		if strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(host, "http://"), "https://"), "/") == DefaultRegistryDomain {
			host = DefaultRegistryHost
		}
		host = strings.TrimRight(host, " ")
		registry.url = GetRegistryUrl(host)
	}
}
