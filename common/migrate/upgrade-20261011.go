package migrate

import (
	"log/slog"
	"net/url"

	"github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/types/define"
)

type Upgrade20261011 struct{}

func (Upgrade20261011) Version() string {
	return "1.11.2"
}

func (Upgrade20261011) Upgrade() error {
	dockerEnv, err := (logic.Env{}).GetEnvByName(define.DockerDefaultClientName)
	if err != nil || dockerEnv == nil {
		return nil
	}
	// 已知连接类型无需迁移；其它值尝试按地址修复。
	switch dockerEnv.RemoteType {
	case define.DockerRemoteTypeSock, define.DockerRemoteTypeTcp, define.DockerRemoteTypeSSH, define.DockerRemoteTypeWSL:
		return nil
	}
	oldRemoteType := dockerEnv.RemoteType
	address, err := url.Parse(dockerEnv.Address)
	if err != nil {
		slog.Warn("cannot infer default Docker remote type", "error", err)
		return nil
	}
	remoteType := ""
	switch address.Scheme {
	case "unix", "npipe":
		if dockerEnv.EnableSSH && dockerEnv.SshServerInfo != nil {
			remoteType = define.DockerRemoteTypeSSH
		} else {
			remoteType = define.DockerRemoteTypeSock
		}
	case "tcp":
		if address.Host != "" {
			remoteType = define.DockerRemoteTypeTcp
		}
	case "wsl":
		if address.Host != "" {
			remoteType = define.DockerRemoteTypeWSL
		}
	case "ssh":
		remoteType = define.DockerRemoteTypeSSH
	}
	if remoteType == "" {
		slog.Warn("cannot infer default Docker remote type", "remoteType", oldRemoteType)
		return nil
	}
	dockerEnv.RemoteType = remoteType
	(logic.Env{}).UpdateEnv(dockerEnv)
	return nil
}
