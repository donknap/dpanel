package registry

import (
	"encoding/base64"
	"fmt"
	"log/slog"
	"strings"

	"github.com/docker/docker/api/types/registry"
	"github.com/donknap/dpanel/common/service/registry/types"
)

const (
	DefaultRegistryDomain = "docker.io"
	DefaultRegistryHost   = "index.docker.io"
)

// {registryUrl}/{namespace-可能有多个路径}/{imageName}:{version}
type ImageTagDetail struct {
	Registry  string
	Namespace string
	ImageName string
	Version   string
	BaseName  string
}

func (self ImageTagDetail) Uri() string {
	if self.Registry == "" {
		self.Registry = DefaultRegistryDomain
	}
	self.Registry = strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(self.Registry, "http://"), "https://"), "/")
	split := ":"
	if self.Namespace == "" {
		return fmt.Sprintf("%s/%s%s%s", self.Registry, self.ImageName, split, self.Version)
	} else {
		return fmt.Sprintf("%s/%s/%s%s%s", self.Registry, self.Namespace, self.ImageName, split, self.Version)
	}
}

func (self ImageTagDetail) Name() string {
	version := self.Version
	if strings.Contains(self.Version, "@") {
		version = strings.Split(version, "@")[0]
	}
	if self.Namespace == "" || self.Namespace == "library" {
		return fmt.Sprintf("%s:%s", self.ImageName, version)
	} else {
		return fmt.Sprintf("%s/%s:%s", self.Namespace, self.ImageName, version)
	}
}

func (self ImageTagDetail) FullName() string {
	if self.Namespace == "" || self.Namespace == "library" {
		return fmt.Sprintf("%s:%s", self.ImageName, self.Version)
	} else {
		return fmt.Sprintf("%s/%s:%s", self.Namespace, self.ImageName, self.Version)
	}
}

type Config struct {
	Username   string   `json:"username"`
	Password   string   `json:"password"`
	Host       string   `json:"host"`
	Proxy      []string `json:"proxy"`
	ExistsAuth bool     `json:"existsAuth"`
}

func (self Config) GetAuthString() string {
	if self.Username == "" || self.Password == "" {
		authString, _ := registry.EncodeAuthConfig(registry.AuthConfig{
			Username: "",
			Password: "",
		})
		return authString
	}
	authString, err := registry.EncodeAuthConfig(registry.AuthConfig{
		Username: self.Username,
		Password: self.Password,
	})
	if err != nil {
		slog.Debug("get registry auth string", err.Error())
		return ""
	}
	return authString
}

func (self Config) GetRegistryAuthString() string {
	if self.Username == "" || self.Password == "" {
		return ""
	}
	return base64.StdEncoding.EncodeToString([]byte(
		fmt.Sprintf("%s:%s",
			self.Username, self.Password,
		)),
	)
}

func (self Config) GetRegistryAuthCredential() types.Credential {
	return types.Credential{
		AccessKey:    self.Username,
		AccessSecret: self.Password,
	}
}

type cacheItem struct {
	body interface{}
}
