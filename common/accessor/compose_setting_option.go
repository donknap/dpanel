package accessor

import (
	"log/slog"
	"os"
	"path/filepath"

	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/docker/types"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/define"
)

const (
	ComposeTypeText        = "text"
	ComposeTypeRemoteUrl   = "remoteUrl"
	ComposeTypeStoragePath = "storagePath"
	ComposeTypeOutPath     = "outPath"
	ComposeTypeDangling    = "dangling"
	ComposeTypeStore       = "store"
	ComposeStatusWaiting   = "waiting"
	ComposeStatusDeploying = "deploying"
	ComposeStatusError     = "error"
)

type ComposeSettingOption struct {
	Status            string          `json:"status,omitempty"`
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

func (self ComposeSettingOption) GetUriFilePath() string {
	if self.Type == ComposeTypeOutPath {
		return self.Uri[0]
	}
	return filepath.Join(self.GetWorkingDir(), self.Uri[0])
}

func (self ComposeSettingOption) GetDefaultEnv() (envFile string, envFileContent []byte, err error) {
	envFile = filepath.Join(filepath.Dir(self.GetUriFilePath()), define.ComposeDefaultEnvFileName)
	// 如果任务中的环境变量值为空，则使用默认 .env 中的值填充
	// 默认情况下，不管 compose 中有没有指定 env_files 都会加载 .env 文件
	// 将面板添加的环境变量通过 -e 参数进行附加, .env 文件使终保持原样
	// 用户修改环境变量时，如果在 .env 文件存在则覆盖文件，否则保存至 setting 中
	_, err = os.Stat(envFile)
	if err != nil {
		return envFile, envFileContent, nil
	}
	envFileContent, err = os.ReadFile(envFile)
	if err != nil {
		return envFile, envFileContent, err
	}
	return envFile, envFileContent, nil
}

func (self ComposeSettingOption) GetWorkingDir() string {
	workDir := storage.Local{}.GetComposePath("")
	if docker.Sdk.DockerEnv.EnableComposePath {
		workDir = storage.Local{}.GetComposePath(docker.Sdk.DockerEnv.Name)
	}
	slog.Debug("compose get container working dir", "dir", workDir)
	return workDir
}

func (self ComposeSettingOption) GetYaml() [2]string {
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
	return yaml
}
