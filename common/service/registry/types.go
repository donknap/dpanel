package registry

import "strings"

type TokenResponse struct {
	Token       string `json:"token"`
	AccessToken string `json:"access_token"`
}

type ImageTagDetail struct {
	Registry  string
	Namespace string
	ImageName string
	Version   string
	Tag       string
	Path      string
}

func (self ImageTagDetail) GetTag() string {
	return self.ImageName[strings.Index(self.ImageName, ":")+1:]
}

func (self ImageTagDetail) GetBaseName() string {
	return self.ImageName[0:strings.Index(self.ImageName, ":")]
}

type ImageTagListResult struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}
