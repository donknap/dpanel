package docker

import (
	"errors"
	"gopkg.in/yaml.v3"
)

type DockerComposeYamlV2 struct {
	Service map[string]ComposeService `yaml:"services"`
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
