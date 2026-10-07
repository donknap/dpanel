package types

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/donknap/dpanel/common/function"
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
	EnableSystemStat  bool            `json:"enableSystemStat,omitempty"`
	ComposePath       string          `json:"composePath,omitempty"`
	EnableSSH         bool            `json:"enableSSH,omitempty"`
	SshServerInfo     *ssh.ServerInfo `json:"sshServerInfo,omitempty"`
	RemoteType        string          `json:"remoteType"`           // 连接客户端类型，支持 sock ssh tcp wsl
	DockerType        string          `json:"dockerType,omitempty"` // 远程客户端类型，docker podman
	DockerInfo        *DockerInfo     `json:"dockerInfo,omitempty"`
	DockerStatus      *DockerStatus   `json:"dockerStatus,omitempty"`
	Enable            *bool           `json:"enable,omitempty"`
}

// IsRemote 按连接类型、地址和面板运行系统判断 Docker 连接是否按远程处理。
// 这里判断的是常见连接拓扑；回环端口若被转发到其他主机，仍会按本地处理。
func (self DockerEnv) IsRemote() bool {
	switch self.RemoteType {
	case define.DockerRemoteTypeWSL:
		// WSL 有独立的 Linux 运行环境，即使从 Windows 本机连接也按远程处理。
		return true
	case define.DockerRemoteTypeSSH:
		// Windows 通过 SSH 访问的文件系统与面板文件系统分开处理。
		if runtime.GOOS == "windows" {
			return true
		}
		// Linux/macOS 上 SSH 到本机回环地址，按本地连接处理。
		if self.SshServerInfo == nil {
			return true
		}
		return !function.IsLoopbackHost(self.SshServerInfo.Address)
	case define.DockerRemoteTypeSock:
		// Unix socket 和 Windows 原始 pipe 都是本机端点，与环境名称无关。
		return false
	}

	address, err := url.Parse(self.Address)
	if err != nil {
		return true
	}
	switch address.Scheme {
	case "unix", "npipe":
		// 未标为 sock 的原始 socket/pipe 仍指向本机；SSH/WSL 代理已在上面处理。
		return false
	case "tcp":
		// Windows、Linux 和 macOS 的本机 TCP 回环端点均按本地处理。
		return !function.IsLoopbackHost(address.Hostname())
	default:
		return true
	}
}

func (self DockerEnv) IsDefault() bool {
	return self.Name == define.DockerDefaultClientName
}

func (self DockerEnv) GetSockName() string {
	name := self.RemoteType + "_" + self.Name
	if runtime.GOOS == "windows" {
		return "npipe:////./pipe/dp_" + name
	}
	return "unix://" + filepath.Join(storage.Local{}.GetLocalProxySockPath(), name+".sock")
}

func (self DockerEnv) CommandEnv() []string {
	result := make([]string, 0)
	if runtime.GOOS == "windows" {
		result = append(result, "COMPOSE_CONVERT_WINDOWS_PATHS=1")
	}
	if self.RemoteType == define.DockerRemoteTypeSSH || self.RemoteType == define.DockerRemoteTypeWSL {
		// 还需要将系统的 PATH 环境变量传递进去，否则可能会报找不到 ssh 命令
		result = append(result, "DOCKER_HOST="+self.GetSockName())
	} else {
		result = append(result, fmt.Sprintf("DOCKER_HOST=%s", self.Address))
		if self.EnableTLS {
			result = append(result,
				"DOCKER_TLS_VERIFY=1",
				"DOCKER_CERT_PATH="+filepath.Dir(filepath.Join(storage.Local{}.GetCertPath(), self.TlsCa)),
			)
		}
	}
	// 只获取指定的系统环境变量，避免其它的污染
	systemEnvList := []string{
		"LANG", "PATH", "HOME", "USER",
		"SHELL", "TERM", "TZ", "PWD",
		"HOSTNAME", "LOGNAME",
		"OLDPWD", "TMPDIR", "TERMINFO_DIRS",
		"COLORTERM", "PAGER", "_",

		"HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY",

		// windows
		"USERPROFILE", "SystemRoot", "APPDATA", "LOCALAPPDATA", "TEMP",
		"TMP", "HOMEDRIVE", "HOMEPATH", "PATHEXT",
		"SystemDrive", "WinDir",

		// @todo dpanel 要用到的环境变量，期待以后修正为以 DP_ 开头
		"STORAGE_LOCAL_PATH", "DP_ACME_CONFIG_HOME", "DB_DATABASE",
		"APP_ENV", "APP_NAME", "APP_FAMILY", "APP_SERVER_PORT", "APP_VERSION",
	}
	result = append(result, function.PluckArrayWalk(os.Environ(), func(item string) (string, bool) {
		ok := false
		for _, s := range systemEnvList {
			if strings.HasPrefix(strings.ToUpper(item), strings.ToUpper(s+"=")) {
				if s == "PATH" {
					// 往 PATH 环境变量中追加程序的目录，便于调用 dpanel 命令
					if v, err := os.Executable(); err == nil {
						item += string(os.PathListSeparator) + filepath.Dir(v)
					}
				}
				ok = true
				break
			}
		}
		return item, ok
	})...)
	return result
}

func (self DockerEnv) CommandParams() []string {
	result := make([]string, 0)
	if self.RemoteType == define.DockerRemoteTypeSSH || self.RemoteType == define.DockerRemoteTypeWSL {
		result = append(result, "-H", self.GetSockName())
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
