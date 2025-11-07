package registry

import (
	"fmt"
	"log/slog"
	"net/url"
	"time"

	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/registry/client"
	"github.com/donknap/dpanel/common/service/registry/client/auth"
	"github.com/donknap/dpanel/common/service/registry/client/auth/bearer"
	"github.com/donknap/dpanel/common/service/registry/types"
	"github.com/donknap/dpanel/common/service/storage"
)

const (
	ChallengeHeader = "WWW-Authenticate"
)

type Registry struct {
	url        url.URL
	credential types.Credential
	authToken  string
	cacheTime  time.Duration
	Client     client.Client
}

func New(opts ...Option) *Registry {
	c := &Registry{}
	for _, opt := range opts {
		opt(c)
	}

	if c.authToken != "" {
		c.Client = client.NewClientWithAuthorizer(c.url.Scheme+"://"+c.url.Host, bearer.NewLocalAuthorizer(c.authToken), true)
	} else {
		c.Client = client.NewClientWithAuthorizer(c.url.Scheme+"://"+c.url.Host, auth.NewAuthorizer(c.credential.AccessKey, c.credential.AccessSecret, true), true)
	}

	return c
}

func (self Registry) GetImageDigest(imageName string) (string, error) {
	imageDetail := GetImageTagDetail(imageName)

	result, err := self.cache("GetImageDigest:"+imageName, func() (interface{}, error) {
		exists, manifest, err := self.Client.ManifestExist(imageDetail.BaseName, imageDetail.Version)
		if err != nil {
			return "", err
		}
		if !exists {
			return "", nil
		}

		return manifest.Digest.String(), nil
	})
	if err != nil {
		return "", err
	}
	return result.(string), nil
}

func (self Registry) GetImageTagList(basename string) ([]string, error) {
	result, err := self.cache("GetImageTagList:"+basename, func() (interface{}, error) {
		tags, err := self.Client.ListTags(basename)
		if err != nil {
			return nil, err
		}
		if tags == nil {
			tags = make([]string, 0)
		}
		return tags, nil
	})
	if err != nil {
		return nil, err
	}
	return result.([]string), nil
}

func (self Registry) cache(serviceName string, handler func() (interface{}, error)) (interface{}, error) {
	cacheKey := fmt.Sprintf("registry:%s:%s:service:%s", docker.Sdk.Name, self.url.Host, serviceName)
	slog.Debug("registry request", "cacheKey", cacheKey)

	if item, ok := storage.Cache.Get(cacheKey); self.cacheTime > 0 && ok {
		if c, ok := item.(cacheItem); ok {
			return c.body, nil
		}
	}

	result, err := handler()
	if err == nil {
		if self.cacheTime > 0 {
			storage.Cache.Set(cacheKey, cacheItem{
				body: result,
			}, self.cacheTime)
		}
	}

	return result, err
}
