package accessor

import (
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/storage"
	"os"
	"path/filepath"
)

const (
	ComposeTypeText        = "text"
	ComposeTypeRemoteUrl   = "remoteUrl"
	ComposeTypeStoragePath = "storagePath"
	ComposeTypeOutPath     = "outPath"
	ComposeTypeDangling    = "dangling"
	ComposeTypeStore       = "store"
	ComposeStatusWaiting   = "waiting"
)

type ComposeSettingOption struct {
	Status            string           `json:"status,omitempty"`
	Type              string           `json:"type"`
	Uri               []string         `json:"uri,omitempty"`
	RemoteUrl         string           `json:"remoteUrl,omitempty"`
	Store             string           `json:"store,omitempty"`
	Environment       []docker.EnvItem `json:"environment,omitempty"`
	DockerEnvName     string           `json:"dockerEnvName,omitempty"`
	DeployServiceName []string         `json:"deployServiceName,omitempty"`
	CreatedAt         string           `json:"createdAt,omitempty"`
	UpdatedAt         string           `json:"updatedAt,omitempty"`
}

func (self ComposeSettingOption) GetUriFilePath() string {
	if self.Type == ComposeTypeOutPath {
		return self.Uri[0]
	}
	return filepath.Join(self.GetWorkingDir(), self.Uri[0])
}

func (self ComposeSettingOption) GetWorkingDir() string {
	if self.DockerEnvName == docker.DefaultClientName {
		return storage.Local{}.GetComposePath()
	} else {
		return filepath.Join(filepath.Dir(storage.Local{}.GetComposePath()), "compose-"+self.DockerEnvName)
	}
}

func (self ComposeSettingOption) GetYaml() ([2]string, error) {
	yaml := [2]string{
		"", "",
	}
	for i, uri := range self.Uri {
		yamlFilePath := ""
		if self.Type == ComposeTypeOutPath {
			// 外部路径分两种，一种是原目录挂载，二是将Yaml文件放置到存储目录中
			if filepath.IsAbs(uri) {
				yamlFilePath = uri
			} else {
				yamlFilePath = filepath.Join(self.GetWorkingDir(), uri)
			}
		} else {
			yamlFilePath = filepath.Join(self.GetWorkingDir(), uri)
		}
		content, err := os.ReadFile(yamlFilePath)
		if err == nil {
			yaml[i] = string(content)
		}
	}
	return yaml, nil
}
