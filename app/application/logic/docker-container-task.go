package logic

import (
	"fmt"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-units"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/notice"
)

func (self DockerTask) ContainerCreate(task *CreateMessage) error {
	_ = notice.Message{}.Info("containerCreate", "正在部署", task.SiteName)
	builder := docker.Sdk.GetContainerCreateBuilder()
	builder.WithImage(task.RunParams.ImageName, false)
	builder.WithContainerName(task.SiteName)

	// 如果绑定了ipv6 需要先创建一个ipv6的自身网络
	if task.RunParams.BindIpV6 || !function.IsEmptyArray(task.RunParams.Links) {
		builder.CreateOwnerNetwork(task.RunParams.BindIpV6)
	}

	// Environment
	if task.RunParams.Environment != nil {
		for _, value := range task.RunParams.Environment {
			if value.Name == "" {
				continue
			}
			builder.WithEnv(value.Name, value.Value)
		}
	}

	// Links
	if !function.IsEmptyArray(task.RunParams.Links) {
		for _, value := range task.RunParams.Links {
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
	if task.RunParams.Ports != nil {
		for _, value := range task.RunParams.Ports {
			builder.WithPort(value.Host, value.Dest)
		}
	}
	if task.RunParams.PublishAllPorts {
		builder.PublishAllPorts()
	}

	// VolumesDefault  Volumes
	if !function.IsEmptyArray(task.RunParams.VolumesDefault) {
		for _, item := range task.RunParams.VolumesDefault {
			if item.Dest == "" {
				continue
			}
			builder.WithDefaultVolume(item.Dest)
		}
	}

	if task.RunParams.Volumes != nil {
		for _, value := range task.RunParams.Volumes {
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
	if task.RunParams.Privileged {
		builder.WithPrivileged()
	}

	// AutoRemove
	if task.RunParams.AutoRemove {
		builder.WithAutoRemove()
	}

	// Restart
	builder.WithRestart(task.RunParams.Restart)

	// cpus
	if task.RunParams.Cpus != 0 {
		builder.WithCpus(task.RunParams.Cpus)
	}

	// memory
	if task.RunParams.Memory != 0 {
		builder.WithMemory(task.RunParams.Memory)
	}

	// shmsize
	if task.RunParams.ShmSize != "" {
		size, _ := units.RAMInBytes(task.RunParams.ShmSize)
		builder.WithShmSize(size)
	}

	// workDir
	if task.RunParams.WorkDir != "" {
		builder.WithWorkDir(task.RunParams.WorkDir)
	}

	if task.RunParams.User != "" {
		builder.WithWorkDir(task.RunParams.WorkDir)
	}

	if task.RunParams.Command != "" {
		builder.WithCommandStr(task.RunParams.Command)
	}

	if task.RunParams.Entrypoint != "" {
		builder.WithEntrypointStr(task.RunParams.Entrypoint)
	}

	if task.RunParams.UseHostNetwork {
		builder.WithNetworkMode(network.NetworkHost)
	}

	if task.RunParams.LogDriver.Driver != "" {
		builder.WithLog(
			task.RunParams.LogDriver.Driver,
			task.RunParams.LogDriver.MaxSize,
			task.RunParams.LogDriver.MaxFile,
		)
	}

	builder.WithDns(task.RunParams.Dns)

	for _, item := range task.RunParams.Label {
		builder.WithLabel(item.Name, item.Value)
	}

	for _, item := range task.RunParams.ExtraHosts {
		builder.WithExtraHosts(item.Name, item.Value)
	}

	response, err := builder.Execute()
	if err != nil {
		dao.Site.Where(dao.Site.ID.Eq(task.SiteId)).Updates(entity.Site{
			Status:  StatusError,
			Message: err.Error(),
		})
		//notice.Message{}.Error("containerCreate", err.Error())
		return err
	}

	// 仅当容器有关联时，才加新建自己的网络。对于ipv6支持，必须加入一个ipv6的网络
	if task.RunParams.BindIpV6 || !function.IsEmptyArray(task.RunParams.Links) {
		err = docker.Sdk.Client.NetworkConnect(docker.Sdk.Ctx, task.SiteName, response.ID, &network.EndpointSettings{
			Aliases: []string{
				fmt.Sprintf("%s.pod.dpanel.local", task.SiteName),
			},
		})
	}

	// 网络需要在创建好容器后统一 connect 否则 bridge 网络会消失。当网络变更后了，可能绑定的端口无法使用。
	// 如果同时绑定多个网络，会以自定义的网络优先，默认的 bridge 网络将不会绑定
	if !function.IsEmptyArray(task.RunParams.Network) {
		for _, value := range task.RunParams.Network {
			if value.Name == task.SiteName {
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
		dao.Site.Where(dao.Site.ID.Eq(task.SiteId)).Updates(entity.Site{
			Status:  StatusError,
			Message: err.Error(),
		})
		//notice.Message{}.Error("containerCreate", err.Error())
		return err
	}

	err = docker.Sdk.Client.ContainerStart(docker.Sdk.Ctx, response.ID, container.StartOptions{})
	if err != nil {
		dao.Site.Where(dao.Site.ID.Eq(task.SiteId)).Updates(entity.Site{
			Status:  StatusError,
			Message: err.Error(),
		})
		//notice.Message{}.Error("containerCreate", err.Error())
		return err
	}

	dao.Site.Where(dao.Site.ID.Eq(task.SiteId)).Updates(&entity.Site{
		ContainerInfo: &accessor.SiteContainerInfoOption{
			ID: response.ID,
		},
		Status:  StatusSuccess,
		Message: "",
	})

	notice.Message{}.Success("containerCreate", task.SiteName)
	return nil
}
