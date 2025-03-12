package registry

import (
	"encoding/base64"
	"fmt"
	"github.com/docker/docker/api/types/registry"
	"log/slog"
	"net/http"
	"strings"
)

const (
	DefaultRegistryDomain = "docker.io"
	DefaultRegistryHost   = "index.docker.io"
)

type TokenResponse struct {
	Token       string `json:"token"`
	AccessToken string `json:"access_token"`
}

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

type ImageTagListResult struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
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

type cacheItem struct {
	header http.Header
	body   []byte
}
