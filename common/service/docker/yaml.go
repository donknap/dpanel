package docker

import (
	"errors"
	"github.com/donknap/dpanel/common/function"
	"gopkg.in/yaml.v3"
	"strings"
)

type DockerComposeYamlV2 struct {
	Service map[string]ComposeService `yaml:"services"`
	XDpanel struct {
		ExternalLinks []string `yaml:"external_links"`
	} `yaml:"x-dpanel"`
	Networks map[string]interface{} `yaml:"networks"`
}

func NewYaml(yamlStr string) (*DockerComposeYamlV2, error) {
	yamlObj := &DockerComposeYamlV2{}
	err := yaml.Unmarshal([]byte(yamlStr), yamlObj)
	if err != nil {
		return nil, err
	}
	return yamlObj, nil
}

type ComposeService struct {
	Image         string   `yaml:"image"`
	Build         string   `yaml:"build"`
	ContainerName string   `yaml:"container_name"`
	Restart       string   `yaml:"restart"`
	Privileged    bool     `yaml:"privileged"`
	Pid           string   `yaml:"pid"`
	VolumesFrom   []string `yaml:"volumes_from"`
	Volumes       []string `yaml:"volumes"`
	Command       []string `yaml:"command"`
	Extend        struct {
		ImageLocalTar map[string]string `yaml:"image_local_tar"`
		AutoRemove    bool              `yaml:"auto_remove"`
	} `yaml:"extend"`
}

func (self DockerComposeYamlV2) GetService(name string) (service *ComposeService, err error) {
	if item, ok := self.Service[name]; ok {
		return &item, nil
	} else {
		return nil, errors.New("service not found")
	}
}

func (self DockerComposeYamlV2) GetServiceName() []string {
	result := make([]string, 0)
	ignore := make([]string, 0)
	for _, item := range self.GetExternalLinks() {
		ignore = append(ignore, item.ServiceName)
	}

	for name, _ := range self.Service {
		if !function.InArray(ignore, name) {
			result = append(result, name)
		}
	}
	return result
}

type linksItem struct {
	ContainerName string
	ServiceName   string
}

func (self DockerComposeYamlV2) GetExternalLinks() []linksItem {
	result := make([]linksItem, 0)
	for _, item := range self.XDpanel.ExternalLinks {
		links := strings.Split(item, ":")
		result = append(result, linksItem{
			ContainerName: links[0],
			ServiceName:   links[1],
		})
	}
	return result
}
