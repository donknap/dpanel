package registry

import (
	"encoding/base64"
	"fmt"
	"strings"
	"time"
)

type Option func(*Registry)

func WithCredentials(username, password string) Option {
	return func(registry *Registry) {
		registry.authString = base64.StdEncoding.EncodeToString([]byte(
			fmt.Sprintf("%s:%s",
				username, password,
			)),
		)
	}
}

func WithCredentialsString(auth string) Option {
	return func(registry *Registry) {
		if auth != "" {
			registry.authString = auth
		}
	}
}

func WithRequestCacheTime(cacheTime time.Duration) Option {
	cache.Range(func(key, value any) bool {
		if value.(cacheItem).expireTime.Before(time.Now()) {
			cache.Delete(key)
		}
		return true
	})
	return func(registry *Registry) {
		registry.cacheTime = cacheTime
	}
}

func WithRegistryHost(host string) Option {
	return func(registry *Registry) {
		host = strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(host, "http://"), "https://"), "/")
		if host == DefaultRegistryDomain {
			host = DefaultRegistryHost
		}
		registry.url = GetRegistryUrl(host)
	}
}
