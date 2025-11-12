package registry

import (
	"log/slog"
	"net/url"

	"github.com/donknap/dpanel/common/service/registry/client"
	"github.com/donknap/dpanel/common/service/registry/client/auth"
	"github.com/donknap/dpanel/common/service/registry/types"
)

func New(opts ...Option) *Registry {
	c := &Registry{}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Client 选取一个可用的地址
func (self Registry) Client() client.Client {
	var c client.Client
	for _, u := range self.address {
		c = client.NewClientWithAuthorizer(u.Scheme+"://"+u.Host, auth.NewAuthorizer(self.credential.AccessKey, self.credential.AccessSecret, true), true)
		if err := c.Ping(); err == nil {
			break
		} else {
			slog.Debug("registry pluck client", "url", c.Address(), "err", err)
		}
	}
	slog.Debug("registry return client", "url", c.Address())
	return c
}

type Registry struct {
	credential types.Credential
	address    []*url.URL
}
