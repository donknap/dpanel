package events

import (
	"log/slog"

	"github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/service/agent/factor"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/types/event"
)

type Plugin struct {
}

func (self Plugin) Destroy(e event.DockerDaemonPayload) {
	if dockerEnv, err := (logic.Env{}).GetEnvByName(e.DockerEnvName); err == nil {
		if dockerSdk, err := docker.NewClientWithDockerEnv(dockerEnv, docker.WithReachabilityCheck()); err == nil {
			if err := factor.DestroyAll(dockerSdk); err != nil {
				slog.Debug("agent destroy", "name", e.DockerEnvName, "error", err)
			}
			if dockerEnv.IsDefault() {
				go func() {
					defer dockerSdk.Close()
					logic.Panel{}.ClearInstallerImage(dockerSdk)
				}()
				return
			}
			dockerSdk.Close()
			return
		}
	}
}
