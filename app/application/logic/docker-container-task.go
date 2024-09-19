package logic

import (
	"fmt"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-units"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/notice"
)

func (self DockerTask) ContainerCreate(task *CreateContainerOption) (string, error) {
	_ = notice.Message{}.Info("containerCreate", "正在部署", task.SiteName)
	builder := docker.Sdk.GetContainerCreateBuilder()
	builder.WithImage(task.BuildParams.ImageName, false)
	builder.WithContainerName(task.SiteName)

	// 如果绑定了ipv6 需要先创建一个ipv6的自身网络
	if task.BuildParams.BindIpV6 || !function.IsEmptyArray(task.BuildParams.Links) {
		builder.CreateOwnerNetwork(task.BuildParams.BindIpV6)
	}

	// Environment
	if task.BuildParams.Environment != nil {
		for _, value := range task.BuildParams.Environment {
			if value.Name == "" {
				continue
			}
			builder.WithEnv(value.Name, value.Value)
		}
	}

	// Links
	if !function.IsEmptyArray(task.BuildParams.Links) {
		for _, value := range task.BuildParams.Links {
			if value.Alise == "" {
				value.Alise = value.Name
			}
			builder.WithLink(value.Name, value.Alise)
			if value.Volume {
				builder.WithContainerVolume(value.Name)
			}
		}
	}

	// Ports PublishAllPorts
	if task.BuildParams.Ports != nil {
		for _, value := range task.BuildParams.Ports {
			builder.WithPort(value.Host, value.Dest)
		}
	}
	if task.BuildParams.PublishAllPorts {
		builder.PublishAllPorts()
	}

	// VolumesDefault  Volumes
	if !function.IsEmptyArray(task.BuildParams.VolumesDefault) {
		for _, item := range task.BuildParams.VolumesDefault {
			if item.Dest == "" {
				continue
			}
			builder.WithDefaultVolume(item.Dest)
		}
	}

	if task.BuildParams.Volumes != nil {
		for _, value := range task.BuildParams.Volumes {
			if value.Host == "" || value.Dest == "" {
				continue
			}
			permission := "rw"
			if value.Permission == "readonly" {
				permission = "ro"
			}
			builder.WithVolume(value.Host, value.Dest, permission)
		}
	}

	// Privileged
	if task.BuildParams.Privileged {
		builder.WithPrivileged()
	}

	// AutoRemove
	if task.BuildParams.AutoRemove {
		builder.WithAutoRemove()
	}

	// Restart
	builder.WithRestart(task.BuildParams.Restart)

	// cpus
	if task.BuildParams.Cpus != 0 {
		builder.WithCpus(task.BuildParams.Cpus)
	}

	// memory
	if task.BuildParams.Memory != 0 {
		builder.WithMemory(task.BuildParams.Memory)
	}

	// shmsize
	if task.BuildParams.ShmSize != "" {
		size, _ := units.RAMInBytes(task.BuildParams.ShmSize)
		builder.WithShmSize(size)
	}

	// workDir
	if task.BuildParams.WorkDir != "" {
		builder.WithWorkDir(task.BuildParams.WorkDir)
	}

	if task.BuildParams.User != "" {
		builder.WithWorkDir(task.BuildParams.WorkDir)
	}

	if task.BuildParams.Command != "" {
		builder.WithCommandStr(task.BuildParams.Command)
	}

	if task.BuildParams.Entrypoint != "" {
		builder.WithEntrypointStr(task.BuildParams.Entrypoint)
	}

	if task.BuildParams.UseHostNetwork {
		builder.WithNetworkMode(network.NetworkHost)
	}

	if task.BuildParams.Log.Driver != "" {
		builder.WithLog(
			task.BuildParams.Log.Driver,
			task.BuildParams.Log.MaxSize,
			task.BuildParams.Log.MaxFile,
		)
	}

	builder.WithDns(task.BuildParams.Dns)

	for _, item := range task.BuildParams.Label {
		builder.WithLabel(item.Name, item.Value)
	}

	for _, item := range task.BuildParams.ExtraHosts {
		builder.WithExtraHosts(item.Name, item.Value)
	}

	response, err := builder.Execute()
	if err != nil {
		return "", err
	}

	// 仅当容器有关联时，才加新建自己的网络。对于ipv6支持，必须加入一个ipv6的网络
	if task.BuildParams.BindIpV6 || !function.IsEmptyArray(task.BuildParams.Links) {
		err = docker.Sdk.Client.NetworkConnect(docker.Sdk.Ctx, task.SiteName, response.ID, &network.EndpointSettings{
			Aliases: []string{
				fmt.Sprintf("%s.pod.dpanel.local", task.SiteName),
			},
		})
	}

	// 网络需要在创建好容器后统一 connect 否则 bridge 网络会消失。当网络变更后了，可能绑定的端口无法使用。
	// 如果同时绑定多个网络，会以自定义的网络优先，默认的 bridge 网络将不会绑定
	if !function.IsEmptyArray(task.BuildParams.Network) {
		for _, value := range task.BuildParams.Network {
			if value.Name == task.SiteName {
				continue
			}
			if value.Name == "host" {
				continue
			}
			for _, aliseName := range value.Alise {
				err = docker.Sdk.Client.NetworkConnect(docker.Sdk.Ctx, value.Name, response.ID, &network.EndpointSettings{
					Aliases: []string{
						aliseName,
					},
				})
			}
		}
	}

	if err != nil {
		//notice.Message{}.Error("containerCreate", err.Error())
		return response.ID, err
	}

	err = docker.Sdk.Client.ContainerStart(docker.Sdk.Ctx, response.ID, container.StartOptions{})
	if err != nil {
		//notice.Message{}.Error("containerCreate", err.Error())
		return response.ID, err
	}
	_ = notice.Message{}.Success("containerCreate", task.SiteName)
	return response.ID, err
}
