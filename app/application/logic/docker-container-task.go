package logic

import (
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/notice"
)

func (self DockerTask) ContainerCreate(task *CreateMessage) error {
	notice.Message{}.Info("containerCreate", "正在部署", task.SiteName)
	builder := docker.Sdk.GetContainerCreateBuilder()
	builder.WithImage(task.RunParams.ImageName, false)
	builder.WithContainerName(task.SiteName)

	if task.RunParams.Ports != nil {
		for _, value := range task.RunParams.Ports {
			builder.WithPort(value.Host, value.Dest)
		}
	}
	if task.RunParams.Environment != nil {
		for _, value := range task.RunParams.Environment {
			builder.WithEnv(value.Name, value.Value)
		}
	}
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
	if !function.IsEmptyArray(task.RunParams.VolumesDefault) {
		for _, item := range task.RunParams.VolumesDefault {
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
	builder.WithRestart(task.RunParams.Restart)

	if task.RunParams.Privileged {
		builder.WithPrivileged()
	}

	if task.RunParams.Cpus != 0 {
		builder.WithCpus(task.RunParams.Cpus)
	}

	if task.RunParams.Memory != 0 {
		builder.WithMemory(task.RunParams.Memory)
	}

	if task.RunParams.ShmSize != 0 {
		builder.WithShmSize(task.RunParams.ShmSize)
	}

	if task.RunParams.WorkDir != "" {
		builder.WithWorkDir(task.RunParams.WorkDir)
	}

	if task.RunParams.User != "" {
		builder.WithWorkDir(task.RunParams.WorkDir)
	}

	if task.RunParams.Command != "" {
		builder.WithCommand(task.RunParams.Command)
	}

	if task.RunParams.Entrypoint != "" {
		builder.WithEntrypoint(task.RunParams.Entrypoint)
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
	// 仅当容器有关联时，才加新建自己的网络
	if !function.IsEmptyArray(task.RunParams.Links) {
		err = docker.Sdk.Client.NetworkConnect(docker.Sdk.Ctx, task.SiteName, response.ID, &network.EndpointSettings{
			Aliases: []string{
				task.SiteName,
			},
		})
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
