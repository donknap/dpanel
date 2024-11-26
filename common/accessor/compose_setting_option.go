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
		yamlFilePath := filepath.Join(storage.Local{}.GetComposePath(), uri)
		content, err := os.ReadFile(yamlFilePath)
		if err != nil {
			return yaml, err
		}
		yaml[i] = string(content)
	}
	return yaml, nil
}
