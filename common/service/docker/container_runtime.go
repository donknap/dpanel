package docker

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/docker/docker/api/types/events"
	types2 "github.com/donknap/dpanel/common/service/docker/types"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/patrickmn/go-cache"
)

const containerRuntimeHistoryLimit = 10

var containerRuntimeMu sync.Mutex

func (self Client) ContainerRuntime(ctx context.Context, containerID string) (types2.ContainerRuntime, bool) {
	select {
	case <-ctx.Done():
		return types2.ContainerRuntime{}, false
	default:
	}

	if v, ok := storage.Cache.Get(self.containerRuntimeCacheKey(containerID)); ok {
		if runtime, ok := v.(types2.ContainerRuntime); ok {
			return runtime, true
		}
	}
	return types2.ContainerRuntime{}, false
}

func (self Client) ContainerRuntimeCollect(ctx context.Context, message events.Message) {
	select {
	case <-ctx.Done():
		return
	default:
	}

	if string(message.Type) != "container" || message.Actor.ID == "" {
		return
	}

	runtimeEvent, ok := self.containerRuntimeEvent(message)
	if !ok {
		return
	}

	containerRuntimeMu.Lock()
	defer containerRuntimeMu.Unlock()

	cacheKey := self.containerRuntimeCacheKey(message.Actor.ID)
	runtime := types2.ContainerRuntime{}
	if v, ok := storage.Cache.Get(cacheKey); ok {
		if cachedRuntime, ok := v.(types2.ContainerRuntime); ok {
			runtime = cachedRuntime
		}
	}

	runtime.ContainerID = message.Actor.ID
	if containerName := message.Actor.Attributes["name"]; containerName != "" {
		runtime.ContainerName = containerName
	}
	runtime.ContainerRuntimeEvent = runtimeEvent
	runtime.History = append(runtime.History, runtimeEvent)
	if len(runtime.History) > containerRuntimeHistoryLimit {
		runtime.History = runtime.History[len(runtime.History)-containerRuntimeHistoryLimit:]
	}

	storage.Cache.Set(cacheKey, runtime, cache.DefaultExpiration)
}

func (self Client) containerRuntimeCacheKey(containerID string) string {
	return fmt.Sprintf(storage.CacheKeyDockerContainerRuntime, self.Name, containerID)
}

func (self Client) containerRuntimeEvent(message events.Message) (types2.ContainerRuntimeEvent, bool) {
	runtimeEvent := types2.ContainerRuntimeEvent{
		Action: string(message.Action),
		Time:   containerRuntimeEventTime(message),
	}

	switch runtimeEvent.Action {
	case "start":
		runtimeEvent.State = "running"
		runtimeEvent.Status = "running"
		runtimeEvent.Running = true
	case "die":
		runtimeEvent.State = "exited"
		runtimeEvent.Status = "exited"
		runtimeEvent.ExitCode, _ = strconv.Atoi(message.Actor.Attributes["exitCode"])
		runtimeEvent.OOMKilled, _ = strconv.ParseBool(message.Actor.Attributes["oomKilled"])
	case "stop":
		runtimeEvent.State = "exited"
		runtimeEvent.Status = "stopped"
	case "kill":
		runtimeEvent.State = "exited"
		runtimeEvent.Status = "killed"
	case "restart":
		runtimeEvent.State = "running"
		runtimeEvent.Status = "restarting"
		runtimeEvent.Running = true
	default:
		return types2.ContainerRuntimeEvent{}, false
	}

	return runtimeEvent, true
}

func containerRuntimeEventTime(message events.Message) time.Time {
	if message.TimeNano > 0 {
		return time.Unix(0, message.TimeNano)
	}
	if message.Time > 0 {
		return time.Unix(message.Time, 0)
	}
	return time.Now()
}
