package logic

import (
	"fmt"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	builder "github.com/donknap/dpanel/common/service/docker/container"
	"github.com/donknap/dpanel/common/service/notice"
	"log/slog"
)

func (self DockerTask) ContainerCreate(task *CreateContainerOption) (string, error) {

	var err error
	var containerOwnerNetwork string

	// 如果绑定了 ipv6 需要先创建一个 ipv6 的自身网络
	// 如果容器配置了 Ip，需要先创建一个自身网络
	// 如果容器关联了其它容器，需要先创建一个自身网络
	if task.BuildParams.BindIpV6 ||
		!function.IsEmptyArray(task.BuildParams.Links) ||
		task.BuildParams.IpV4 != nil || task.BuildParams.IpV6 != nil {
		// 删除掉的网络
		err = docker.Sdk.NetworkRemove(task.SiteName)
		if err != nil {
			return "", err
		}
		containerOwnerNetwork, err = docker.Sdk.NetworkCreate(task.SiteName, task.BuildParams.IpV4, task.BuildParams.IpV6)
		if err != nil {
			return "", err
		}
	}
	options := make([]builder.Option, 0)

	if oldContainerInfo, err := docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, task.SiteName); err == nil {
		_ = notice.Message{}.Info(".containerRemove", task.SiteName)
		if oldContainerInfo.State.Running {
			err = docker.Sdk.Client.ContainerStop(docker.Sdk.Ctx, oldContainerInfo.ID, container.StopOptions{})
			if err != nil {
				return "", err
			}
		}
		err = docker.Sdk.Client.ContainerRemove(docker.Sdk.Ctx, oldContainerInfo.ID, container.RemoveOptions{})
		if err != nil {
			return "", err
		}
		options = append(options, builder.WithContainerInfo(oldContainerInfo))
	}

	options = append(options, []builder.Option{
		builder.WithContainerName(task.SiteName),
		builder.WithHostname(task.BuildParams.Hostname),
		builder.WithDomainName(fmt.Sprintf(docker.HostnameTemplate, task.SiteName)),
		builder.WithImage(task.BuildParams.ImageName, false),
		builder.WithEnv(task.BuildParams.Environment...),
		builder.WithVolumesFrom(task.BuildParams.Links...),
		builder.WithPort(task.BuildParams.Ports...),
		builder.WithPublishAllPorts(task.BuildParams.PublishAllPorts),
		builder.WithVolume(task.BuildParams.Volumes...),
		builder.WithPrivileged(task.BuildParams.Privileged),
		builder.WithAutoRemove(task.BuildParams.AutoRemove),
		builder.WithRestartPolicy(task.BuildParams.Restart),
		builder.WithCpus(task.BuildParams.Cpus),
		builder.WithMemory(task.BuildParams.Memory),
		builder.WithShmSize(task.BuildParams.ShmSize),
		builder.WithWorkDir(task.BuildParams.WorkDir),
		builder.WithUser(task.BuildParams.User),
		builder.WithCommandStr(task.BuildParams.Command),
		builder.WithEntrypointStr(task.BuildParams.Entrypoint),
		builder.WithLog(task.BuildParams.Log),
		builder.WithDns(task.BuildParams.Dns),
		builder.WithLabel(task.BuildParams.Label...),
		builder.WithExtraHosts(task.BuildParams.ExtraHosts...),
		builder.WithDevice(task.BuildParams.Device...),
		builder.WithGpus(task.BuildParams.Gpus),
		builder.WithHealthcheck(task.BuildParams.Healthcheck),
		builder.WithCap(task.BuildParams.CapAdd...),
	}...)

	if task.BuildParams.HostPid {
		options = append(options, builder.WithHostPid())
	}
	if task.BuildParams.UseHostNetwork {
		options = append(options, builder.WithHostNetwork())
	}

	useBridgeNetwork := function.InArrayWalk(task.BuildParams.Network, func(i docker.NetworkItem) bool {
		if i.Name == network.NetworkBridge {
			return true
		}
		return false
	})
	// 如果没有开启加入 bridge 网络，在创建时添加网络参数
	if !useBridgeNetwork && !function.IsEmptyArray(task.BuildParams.Network) {
		options = append(options, builder.WithNetwork(task.BuildParams.Network...))
	}

	b, err := builder.New(options...)
	if err != nil {
		return "", err
	}

	_ = notice.Message{}.Info(".containerCreate", task.SiteName)
	response, err := b.Execute()
	if err != nil {
		return "", err
	}

	err = docker.Sdk.Client.ContainerStart(docker.Sdk.Ctx, response.ID, container.StartOptions{})
	if err != nil {
		return response.ID, err
	}

	// 当前如果新建了容器自身网络，创建完后加入
	// 如果在创建时加入，则会丢失 bridge 网络
	if containerOwnerNetwork != "" {
		o := docker.NetworkItem{
			Name: containerOwnerNetwork,
		}
		if task.BuildParams.IpV6 != nil {
			o.IpV6 = task.BuildParams.IpV6.Address
		}
		if task.BuildParams.IpV4 != nil {
			o.IpV4 = task.BuildParams.IpV4.Address
		}
		err = docker.Sdk.NetworkConnect(o, task.SiteName)
		if err != nil {
			return "", err
		}
	}

	// 利用Network关联容器
	// 每次创建自身网络时，先删除掉，最后再统一将关联和自身加入进来
	// 容器关联时必须采用 hostname 以保证容器可以访问
	if !function.IsEmptyArray(task.BuildParams.Links) {
		for _, value := range task.BuildParams.Links {
			if value.Alise == "" {
				value.Alise = value.Name
			}
			err = docker.Sdk.NetworkConnect(docker.NetworkItem{
				Name: containerOwnerNetwork,
				Alise: []string{
					value.Alise,
				},
			}, value.Name)
			if err != nil {
				return "", err
			}
		}
	}

	// 网络需要在创建好容器后统一 connect 否则 bridge 网络会消失。当网络变更后了，可能绑定的端口无法使用。
	// 如果同时绑定多个网络，会以自定义的网络优先，默认的 bridge 网络将不会绑定
	if useBridgeNetwork && !function.IsEmptyArray(task.BuildParams.Network) {
		for _, value := range task.BuildParams.Network {
			if function.InArray([]string{
				network.NetworkDefault,
				network.NetworkHost,
				network.NetworkNone,
				network.NetworkBridge,
				network.NetworkNat,
			}, value.Name) {
				continue
			}
			err = docker.Sdk.NetworkConnect(value, task.SiteName)
			if err != nil {
				return "", err
			}
		}
	}

	if task.BuildParams.Hook != nil && task.BuildParams.Hook.ContainerCreate != "" {
		_, err := docker.Sdk.ExecResult(response.ID, task.BuildParams.Hook.ContainerCreate)
		if err != nil {
			slog.Debug("container create run hook", "hook", "container create", "error", err.Error())
		}
	}

	return response.ID, err
}
