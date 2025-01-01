package registry

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
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

func WithRegistryHost(host string) Option {
	return func(registry *Registry) {
		host = strings.TrimPrefix(strings.TrimPrefix(host, "http://"), "https://")
		if host == DefaultRegistryDomain {
			host = DefaultRegistryHost
		}
		registry.url = url.URL{
			Scheme: "https",
			Host:   host,
			Path:   "/v2/",
		}
	}
}
