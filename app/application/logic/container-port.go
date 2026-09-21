package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	agentTypes "github.com/donknap/dpanel/app/agent/types"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/plugin"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/patrickmn/go-cache"
)

const (
	containerPortCheckSuccess = "success"
	containerPortCheckFailed  = "failed"
	containerPortCheckTimeout = "timeout"

	containerPortCheckTimeoutDuration = 3 * time.Second
	containerPortCheckWorkers         = 16
)

type ContainerPort struct{}

type ContainerPortCheckItem struct {
	Address string `json:"address"`
	Status  string `json:"status"`
}

func (ContainerPort) Check(ctx context.Context, ports *[]ContainerPortCheckItem) error {
	items := *ports

	jobs := make(chan int)
	workerTotal := min(containerPortCheckWorkers, len(items))
	var workers sync.WaitGroup
	workers.Add(workerTotal)
	for range workerTotal {
		go func() {
			defer workers.Done()
			for index := range jobs {
				checkCtx, cancel := context.WithTimeout(ctx, containerPortCheckTimeoutDuration)
				err := function.CheckTCP(checkCtx, items[index].Address)
				cancel()
				status := containerPortCheckSuccess
				if err != nil {
					status = containerPortCheckFailed
					var netErr net.Error
					if errors.Is(err, context.DeadlineExceeded) || errors.As(err, &netErr) && netErr.Timeout() {
						status = containerPortCheckTimeout
					}
				}
				items[index].Status = status
			}
		}()
	}
	for index := range items {
		select {
		case jobs <- index:
		case <-ctx.Done():
			close(jobs)
			workers.Wait()
			return ctx.Err()
		}
	}
	close(jobs)
	workers.Wait()
	return ctx.Err()
}

func (ContainerPort) Find(ctx context.Context, dockerSdk *docker.Client, containerIDs []string, hostNetworkOnly bool) (err error) {
	results := make(map[string]agentTypes.PortResult, len(containerIDs))
	targets := make(map[string]struct{}, len(containerIDs))
	command := []string{"/agent", "port"}
	for _, id := range containerIDs {
		info, err := dockerSdk.Client.ContainerInspect(ctx, id)
		if err != nil {
			return fmt.Errorf("inspect container %s: %w", id, err)
		}
		results[id] = agentTypes.PortResult{
			ContainerID: id,
			Ports:       make([]agentTypes.Port, 0),
		}
		if info.State != nil && info.State.Running && !info.State.Paused && !info.State.Restarting && info.State.Pid > 1 &&
			info.HostConfig != nil && info.HostConfig.NetworkMode != network.NetworkNone &&
			(!hostNetworkOnly || info.HostConfig.NetworkMode == network.NetworkHost) {
			command = append(command, "--container-id", fmt.Sprintf("%s:%d", id, info.State.Pid))
			targets[id] = struct{}{}
		}
	}

	if len(targets) > 0 {
		agent, err := plugin.NewPlugin(dockerSdk, plugin.MonitorName, plugin.CreateOption{
			Init:            true,
			MountDockerRoot: dockerSdk.DockerEnv != nil && dockerSdk.DockerEnv.EnableSystemStat,
		})
		if err != nil {
			return err
		}
		if err = agent.Create(); err != nil {
			return err
		}
		output, err := dockerSdk.ContainerExecResult(ctx, plugin.MonitorName, container.ExecOptions{Cmd: command})
		if err != nil {
			return fmt.Errorf("execute port detection agent: %w", err)
		}

		responses := make(map[string]agentTypes.PortResult, len(targets))
		decoder := json.NewDecoder(strings.NewReader(output))
		decoder.DisallowUnknownFields()
		for {
			response := agentTypes.PortResult{}
			if err = decoder.Decode(&response); errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return fmt.Errorf("decode port detection agent response: %w", err)
			}
			if _, exists := targets[response.ContainerID]; !exists {
				return fmt.Errorf("port detection agent returned unknown container %s", response.ContainerID)
			}
			if _, exists := responses[response.ContainerID]; exists {
				return fmt.Errorf("port detection agent returned duplicate container %s", response.ContainerID)
			}
			if response.Error != "" {
				return fmt.Errorf("detect container %s ports: %s", response.ContainerID, response.Error)
			}
			if response.Ports == nil {
				response.Ports = make([]agentTypes.Port, 0)
			}
			responses[response.ContainerID] = response
		}
		if len(responses) != len(targets) {
			return errors.New("port detection agent did not return all containers")
		}
		for id, response := range responses {
			results[id] = response
		}
	}

	for _, id := range containerIDs {
		storage.Cache.Set(
			fmt.Sprintf(storage.CacheKeyDockerContainerPort, dockerSdk.Name, id),
			results[id],
			cache.DefaultExpiration,
		)
	}
	return nil
}
