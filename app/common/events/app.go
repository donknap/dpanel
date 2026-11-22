package events

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/service/agent/factor"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/docker/types"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/donknap/dpanel/common/types/event"
)

type App struct{}

func (self App) Bootstrap(e event.AppBootstrapPayload) {
	dockerEnv := e.DockerEnvList[define.DockerDefaultClientName]
	if dockerEnv == nil || (dockerEnv.Enable != nil && !*dockerEnv.Enable) {
		return
	}
	dockerSdk, err := docker.NewClientWithDockerEnv(dockerEnv, docker.WithReachabilityCheck())
	if err != nil {
		slog.Warn("get Docker client for installer image cleanup", "name", dockerEnv.Name, "error", err)
		return
	}
	defer dockerSdk.Close()
	logic.Panel{}.ClearInstallerImage(dockerSdk)
}

func (self App) Shutdown() {
	slog.Info("app shutdown")
	dockerEnvList := make(map[string]*types.DockerEnv)
	logic.Setting{}.GetByKey(logic.SettingGroupSetting, logic.SettingGroupSettingDocker, &dockerEnvList)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var waitGroup sync.WaitGroup
	for _, env := range dockerEnvList {
		if env.Enable != nil && !*env.Enable {
			continue
		}
		// EventBus 同步回调持有总线锁，不能在回调内再次 Publish。
		waitGroup.Add(1)
		go func(dockerEnv *types.DockerEnv) {
			defer waitGroup.Done()
			dockerSdk, err := docker.NewClientWithDockerEnv(dockerEnv, docker.WithReachabilityCheck())
			if err != nil {
				slog.Warn("get Docker client for shutdown cleanup", "name", dockerEnv.Name, "error", err)
				return
			}
			defer dockerSdk.Close()
			if err := factor.DestroyAll(ctx, dockerSdk); err != nil {
				slog.Warn("agent shutdown cleanup", "name", dockerEnv.Name, "error", err)
			}
		}(env)
	}
	done := make(chan struct{})
	go func() {
		waitGroup.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		slog.Warn("app shutdown cleanup timeout", "error", ctx.Err())
	}
}
