package registry

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/mcuadros/go-version"
	"io"
	"net/http"
	"sort"
)

const ContentDigestHeader = "Docker-Content-Digest"

type repository struct {
	registry *Registry
}

func (self repository) GetImageDigest(imageName string) (string, error) {
	imageDetail := GetImageTagDetail(imageName)
	token, err := self.registry.accessToken(fmt.Sprintf("repository:%s:pull", imageDetail.GetBaseName()))
	if err != nil {
		return "", err
	}
	u := self.registry.url.JoinPath(imageDetail.GetBaseName(), "manifests", imageDetail.GetTag())
	req, _ := http.NewRequest("HEAD", u.String(), nil)
	req.Header.Add("Accept", "application/vnd.docker.distribution.manifest.v2+json")
	req.Header.Add("Accept", "application/vnd.docker.distribution.manifest.list.v2+json")
	req.Header.Add("Accept", "application/vnd.docker.distribution.manifest.v1+json")
	req.Header.Add("Accept", "application/vnd.oci.image.index.v1+json")
	req.Header.Set("Authorization", token)
	res, err := self.registry.request(req)
	if err != nil {
		return "", err
	}
	return res.Header.Get(ContentDigestHeader), nil
}

func (self repository) GetImageTagList(imageName string) ([]string, error) {
	imageDetail := GetImageTagDetail(imageName)
	token, err := self.registry.accessToken(fmt.Sprintf("repository:%s:pull", imageDetail.GetBaseName()))
	if err != nil {
		return nil, err
	}
	u := self.registry.url.JoinPath(imageDetail.GetBaseName(), "tags", "list")
	req, _ := http.NewRequest("GET", u.String(), nil)
	req.Header.Set("Authorization", token)
	res, err := self.registry.request(req)
	if err != nil {
		return nil, err
	}
	buffer := new(bytes.Buffer)
	_, err = io.Copy(buffer, res.Body)
	if err != nil {
		return nil, err
	}
	result := ImageTagListResult{}
	err = json.Unmarshal(buffer.Bytes(), &result)
	if err != nil {
		return nil, err
	}
	sort.Slice(result.Tags, func(i, j int) bool {
		return version.CompareSimple(result.Tags[i], result.Tags[j]) == 1
	})
	return result.Tags, nil
}
