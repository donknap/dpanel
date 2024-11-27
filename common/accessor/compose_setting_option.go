package accessor

import (
	"github.com/donknap/dpanel/common/service/storage"
	"os"
	"path/filepath"
)

const (
	ComposeTypeText        = "text"
	ComposeTypeRemoteUrl   = "remoteUrl"
	ComposeTypeStoragePath = "storagePath"
	ComposeTypeOutPath     = "outPath"
	ComposeTypeStore       = "store"
	ComposeStatusWaiting   = "waiting"
)

type ComposeSettingOption struct {
	Status      string    `json:"status,omitempty"`
	Type        string    `json:"type"`
	Uri         []string  `json:"uri,omitempty"`
	RemoteUrl   string    `json:"remoteUrl,omitempty"`
	Store       string    `json:"store,omitempty"`
	Environment []EnvItem `json:"environment,omitempty"`
}

func (self ComposeSettingOption) GetUriFilePath() string {
	return filepath.Join(storage.Local{}.GetComposePath(), self.Uri[0])
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
				yamlFilePath = filepath.Join(storage.Local{}.GetComposePath(), uri)
			}
		} else {
			yamlFilePath = filepath.Join(storage.Local{}.GetComposePath(), uri)
		}
		content, err := os.ReadFile(yamlFilePath)
		if err == nil {
			yaml[i] = string(content)
		}
	}
	return yaml, nil
}
