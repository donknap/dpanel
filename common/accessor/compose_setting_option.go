package accessor

import (
	"errors"
	"fmt"
	"path"
	"path/filepath"

	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker/types"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/define"
)

const (
	ComposeTypeText        = "text"
	ComposeTypeRemoteUrl   = "remoteUrl"
	ComposeTypeStoragePath = "storagePath"
	ComposeTypeOutPath     = "outPath"
	ComposeTypeStore       = "store"
	ComposeStatusWaiting   = "waiting"
	ComposeStatusDeploying = "deploying"
	ComposeStatusError     = "error"
)

var ComposeFileNameSuffix = []string{
	"docker-compose.yml", "docker-compose.yaml",
	"compose.yml", "compose.yaml",
}

type ComposeSettingOption struct {
	Status            string          `json:"status,omitempty"`
	Dangling          bool            `json:"dangling"`
	Delegate          *bool           `json:"delegate"` // 配置委托给面板，原 Uri 仍作为同步目标。
	Type              string          `json:"type"`
	Uri               []string        `json:"uri,omitempty"`
	RemoteUrl         string          `json:"remoteUrl,omitempty"`
	Store             string          `json:"store,omitempty"`
	Environment       []types.EnvItem `json:"environment,omitempty"`
	DockerEnvName     string          `json:"dockerEnvName,omitempty"`
	DeployServiceName []string        `json:"deployServiceName,omitempty"`
	CreatedAt         string          `json:"createdAt,omitempty"`
	UpdatedAt         string          `json:"updatedAt,omitempty"`
	Message           string          `json:"message,omitempty"`
	RunName           string          `json:"-"` // Deprecated: 兼容旧版有前缀的名称
}

func (self ComposeSettingOption) IsDelegate() bool {
	return self.Type == ComposeTypeOutPath && self.Delegate != nil && *self.Delegate
}

func (self ComposeSettingOption) GetUriPath(composeName string) (string, string, error) {
	if !function.IsSafeString(composeName, function.StringRuleName) || len(self.Uri) == 0 || self.Uri[0] == "" {
		return "", "", errors.New("invalid compose configuration path")
	}
	if self.Type == ComposeTypeOutPath {
		composeFile := function.SlashPathToSystem(self.Uri[0])
		overrideFileName := define.ComposeProjectDeployOverrideFileName
		if self.IsDelegate() {
			composeFileName := path.Base(self.Uri[0])
			if !filepath.IsLocal(composeFileName) || composeFileName == "." {
				return "", "", errors.New("invalid compose configuration path")
			}
			composeFile = filepath.Join(storage.Local{}.GetComposeProjectPath(self.DockerEnvName, composeName), composeFileName)
		} else {
			overrideFileName = fmt.Sprintf(define.ComposeProjectDeployOverrideOutPathFileName, composeName)
		}
		return composeFile, filepath.Join(filepath.Dir(composeFile), overrideFileName), nil
	}
	composeUri := function.SlashPathToSystem(self.Uri[0])
	for _, uri := range self.Uri {
		if !filepath.IsLocal(uri) {
			return "", "", fmt.Errorf("invalid compose URI %q", uri)
		}
		uri = function.SlashPathToSystem(uri)
		if filepath.Dir(uri) != filepath.Dir(composeUri) {
			return "", "", errors.New("compose files must share a directory")
		}
	}
	composeFile := filepath.Join(storage.Local{}.GetComposePath(self.DockerEnvName), composeUri)
	return composeFile, filepath.Join(filepath.Dir(composeFile), define.ComposeProjectDeployOverrideFileName), nil
}
