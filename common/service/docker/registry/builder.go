package registry

import (
	"log/slog"

	dockerRegistry "github.com/docker/docker/api/types/registry"
)

func NewRegistry(opt ...Option) *Registry {
	s := &Registry{
		Address: make([]string, 0),
		Config:  dockerRegistry.AuthConfig{},
	}

	for _, option := range opt {
		option(s)
	}

	return s
}

type Registry struct {
	Address []string
	Config  dockerRegistry.AuthConfig
}

func (self Registry) GetAuthString() string {
	authString, err := dockerRegistry.EncodeAuthConfig(self.Config)
	if err != nil {
		slog.Debug("get registry auth string", err.Error())
		return ""
	}
	return authString
}
