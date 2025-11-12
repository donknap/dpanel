package registry

import (
	"net/url"
	"strings"

	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/registry/types"
	"github.com/donknap/dpanel/common/types/define"
)

type Option func(*Registry)

func WithCredentials(credential types.Credential) Option {
	return func(registry *Registry) {
		registry.credential = credential
	}
}

func WithCredentialsWithBasic(username, password string) Option {
	return WithCredentials(types.Credential{
		AccessKey:    username,
		AccessSecret: password,
	})
}

func WithAddress(address ...string) Option {
	return func(registry *Registry) {
		registry.address = function.PluckArrayWalk(address, func(s string) (u *url.URL, ok bool) {
			// docker.io 地址转换成仓库需使用 index.docker.io
			if strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(s, "http://"), "https://"), "/") == define.RegistryDefaultName {
				s = define.RegistryDefaultHost
			}
			if !strings.HasPrefix(s, "http") {
				s = "https://" + s
			}
			u, err := url.Parse(s)
			if err != nil {
				return u, false
			}
			return u, true
		})
	}
}
