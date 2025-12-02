package types

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/donknap/dpanel/common/service/ssh"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/define"
)

type DockerEnv struct {
	Name              string          `json:"name,omitempty" binding:"required"`
	Title             string          `json:"title,omitempty" binding:"required"`
	Address           string          `json:"address,omitempty" binding:"required"` // docker api 地址
	Default           bool            `json:"default,omitempty"`                    // 是否是默认客户端
	ServerUrl         string          `json:"serverUrl,omitempty"`
	EnableTLS         bool            `json:"enableTLS,omitempty"`
	TlsCa             string          `json:"tlsCa,omitempty"`
	TlsCert           string          `json:"tlsCert,omitempty"`
	TlsKey            string          `json:"tlsKey,omitempty"`
	EnableComposePath bool            `json:"enableComposePath,omitempty"` // 启用 compose 独享目录
	ComposePath       string          `json:"composePath,omitempty"`
	EnableSSH         bool            `json:"enableSSH,omitempty"`
	SshServerInfo     *ssh.ServerInfo `json:"sshServerInfo,omitempty"`
	RemoteType        string          `json:"remoteType"` // 远程客户端类型，支持 docker ssh
	DockerInfo        *DockerInfo     `json:"dockerInfo,omitempty"`
	DockerStatus      *DockerStatus   `json:"dockerStatus,omitempty"`
}

func (self DockerEnv) CommandEnv() []string {
	result := make([]string, 0)
	if self.RemoteType == define.DockerRemoteTypeSSH {
		// 还需要将系统的 PATH 环境变量传递进去，否则可能会报找不到 ssh 命令
		if runtime.GOOS == "windows" {
			result = append(result, fmt.Sprintf("DOCKER_HOST=npipe:////./pipe/dp_%s", self.Name))
		} else {
			result = append(result, fmt.Sprintf("DOCKER_HOST=unix://%s/%s.sock", storage.Local{}.GetLocalProxySockPath(), self.Name))
		}
		result = append(result, os.Environ()...)
		return result
	}
	result = append(result, fmt.Sprintf("DOCKER_HOST=%s", self.Address))
	if self.EnableTLS {
		result = append(result,
			"DOCKER_TLS_VERIFY=1",
			"DOCKER_CERT_PATH="+filepath.Dir(filepath.Join(storage.Local{}.GetCertPath(), self.TlsCa)),
		)
	}
	return result
}

func (self DockerEnv) CommandParams() []string {
	result := make([]string, 0)
	if self.RemoteType == define.DockerRemoteTypeSSH {
		if runtime.GOOS == "windows" {
			result = append(result, "-H", fmt.Sprintf("npipe:////./pipe/dp_%s", self.Name))
		} else {
			result = append(result, "-H", fmt.Sprintf("unix://%s/%s.sock", storage.Local{}.GetLocalProxySockPath(), self.Name))
		}
		return result
	}
	result = append(result, "-H", self.Address)
	if self.EnableTLS {
		result = append(result, "--tlsverify",
			"--tlscacert", filepath.Join(storage.Local{}.GetCertPath(), self.TlsCa),
			"--tlscert", filepath.Join(storage.Local{}.GetCertPath(), self.TlsCert),
			"--tlskey", filepath.Join(storage.Local{}.GetCertPath(), self.TlsKey),
		)
	}
	return result
}

func (self DockerEnv) CertRoot() string {
	return filepath.Join("docker", self.Name)
}
