package logic

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	agentTypes "github.com/donknap/dpanel/app/agent/types"
	statLogic "github.com/donknap/dpanel/app/common/logic/stat"
	"github.com/donknap/dpanel/common/function"
	serviceAgent "github.com/donknap/dpanel/common/service/agent"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/plugin"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/patrickmn/go-cache"
)

type ContainerPort struct{}

const (
	containerPortCheckMaxContainers = 256
	containerPortCheckMaxPorts      = 4096
)

func (ContainerPort) CheckablePorts(info container.InspectResponse) ([]string, bool) {
	if info.State == nil || !info.State.Running || info.HostConfig == nil ||
		info.HostConfig.NetworkMode == network.NetworkNone {
		return nil, false
	}
	if info.Config != nil {
		if hidden, exists := info.Config.Labels[define.DPanelLabelContainerHidden]; exists &&
			(hidden == "true" || hidden == "1") {
			return nil, false
		}
	}
	if info.HostConfig.NetworkMode == network.NetworkHost {
		return make([]string, 0), true
	}
	if info.NetworkSettings == nil {
		return nil, false
	}
	ports := make([]string, 0)
	seenPorts := make(map[string]struct{})
	for port, bindings := range info.NetworkSettings.Ports {
		protocol := strings.ToLower(port.Proto())
		if protocol != "tcp" && protocol != "udp" {
			continue
		}
		for _, binding := range bindings {
			value, err := strconv.ParseUint(binding.HostPort, 10, 16)
			if err != nil || value == 0 {
				continue
			}
			family := "4"
			if strings.Contains(binding.HostIP, ":") {
				family = "6"
			}
			address := fmt.Sprintf("%d/%s%s", value, protocol, family)
			if _, exists := seenPorts[address]; exists {
				continue
			}
			seenPorts[address] = struct{}{}
			ports = append(ports, address)
		}
	}
	if len(ports) == 0 {
		return nil, false
	}
	sort.Strings(ports)
	return ports, true
}

func (ContainerPort) Check(
	ctx context.Context, dockerSdk *docker.Client, targets []serviceAgent.PortCheckTarget,
) ([]agentTypes.PortCheckResult, error) {
	if dockerSdk.DockerEnv == nil || !dockerSdk.DockerEnv.EnableSystemStat {
		return nil, function.ErrorMessage(define.ErrorMessageContainerPortRequiresSystemStat)
	}
	if len(targets) > containerPortCheckMaxContainers {
		return nil, fmt.Errorf("container port check supports at most %d containers", containerPortCheckMaxContainers)
	}
	portTotal := 0
	for _, target := range targets {
		portTotal += len(target.Ports)
		if portTotal > containerPortCheckMaxPorts {
			return nil, fmt.Errorf("container port check supports at most %d ports", containerPortCheckMaxPorts)
		}
	}
	if err := (statLogic.Stat{}).ReconcileSystemStat(dockerSdk); err != nil {
		return nil, err
	}
	agent, err := serviceAgent.NewDockerAgent(dockerSdk, plugin.MonitorName)
	if err != nil {
		return nil, err
	}
	result, err := agent.CheckPorts(ctx, targets)
	if err != nil {
		return nil, err
	}
	for _, item := range result {
		storage.Cache.Set(
			fmt.Sprintf(storage.CacheKeyDockerContainerPort, dockerSdk.Name, item.ContainerID),
			item,
			cache.DefaultExpiration,
		)
	}
	return result, nil
}

func (ContainerPort) LoadCache(
	dockerName string, targets []serviceAgent.PortCheckTarget,
) []agentTypes.PortCheckResult {
	result := make([]agentTypes.PortCheckResult, 0, len(targets))
	for _, target := range targets {
		item, exists := storage.LoadCache[agentTypes.PortCheckResult](
			fmt.Sprintf(storage.CacheKeyDockerContainerPort, dockerName, target.ContainerID),
		)
		if !exists {
			item = agentTypes.PortCheckResult{
				ContainerID: target.ContainerID,
				Ports:       make([]agentTypes.PortCheckItem, 0),
			}
		}
		result = append(result, item)
	}
	return result
}
