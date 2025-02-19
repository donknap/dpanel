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

	u := self.registry.url.JoinPath(imageDetail.BaseName, "manifests", imageDetail.Version)
	req, err := http.NewRequest("HEAD", u.String(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Add("Accept", "application/vnd.docker.distribution.manifest.v2+json")
	req.Header.Add("Accept", "application/vnd.docker.distribution.manifest.list.v2+json")
	req.Header.Add("Accept", "application/vnd.docker.distribution.manifest.v1+json")
	req.Header.Add("Accept", "application/vnd.oci.image.index.v1+json")

	res, err := self.registry.request(req, fmt.Sprintf(ScopeRepositoryPull, imageDetail.BaseName))
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	return res.Header.Get(ContentDigestHeader), nil
}

func (self repository) GetImageTagList(basename string) ([]string, error) {
	u := self.registry.url.JoinPath(basename, "tags", "list")
	//if limit > 0 {
	//	u.RawQuery = fmt.Sprintf("n=%d", limit)
	//}
	req, _ := http.NewRequest("GET", u.String(), nil)

	res, err := self.registry.request(req, fmt.Sprintf(ScopeRepositoryPull, basename))
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

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
