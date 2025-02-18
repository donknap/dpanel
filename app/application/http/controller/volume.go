package controller

import (
	"fmt"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/go-units"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/notice"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
	"sort"
	"strings"
)

type Volume struct {
	controller.Abstract
}

func (self Volume) GetList(http *gin.Context) {
	type ParamsValidate struct {
		Name string `json:"name"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	filter := filters.NewArgs()
	if params.Name != "" {
		filter.Add("name", params.Name)
	}
	volumeList, err := docker.Sdk.Client.VolumeList(docker.Sdk.Ctx, volume.ListOptions{
		Filters: filter,
	})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	type useVolumeContainerItem struct {
		Name string `json:"name"`
		Md5  string `json:"md5"`
	}
	inUseVolume := make(map[string][]useVolumeContainerItem)
	containerList, err := docker.Sdk.Client.ContainerList(docker.Sdk.Ctx, container.ListOptions{
		All:    true,
		Latest: true,
	})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	for _, item := range containerList {
		for _, mount := range item.Mounts {
			if mount.Name != "" {
				if _, ok := inUseVolume[mount.Name]; !ok {
					inUseVolume[mount.Name] = make([]useVolumeContainerItem, 0)
				}
				inUseVolume[mount.Name] = append(inUseVolume[mount.Name], useVolumeContainerItem{
					Name: item.Names[0],
					Md5:  item.ID,
				})
			}
		}
	}

	sort.Slice(volumeList.Volumes, func(i, j int) bool {
		return volumeList.Volumes[i].CreatedAt > volumeList.Volumes[j].CreatedAt
	})
	self.JsonResponseWithoutError(http, gin.H{
		"list":    volumeList.Volumes,
		"warning": volumeList.Warnings,
		"inUse":   inUseVolume,
	})
	return
}

func (self Volume) GetDetail(http *gin.Context) {
	type ParamsValidate struct {
		Name string `json:"name" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	volumeInfo, err := docker.Sdk.Client.VolumeInspect(docker.Sdk.Ctx, params.Name)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	type useContainer struct {
		Id    string
		Name  string
		Mount string
		RW    bool
	}
	var inUseContainer []useContainer
	containerList, err := docker.Sdk.Client.ContainerList(docker.Sdk.Ctx, container.ListOptions{
		All:    true,
		Latest: true,
	})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	for _, item := range containerList {
		for _, mount := range item.Mounts {
			if mount.Name != "" && mount.Name == params.Name {
				inUseContainer = append(inUseContainer, useContainer{
					Name:  item.Names[0],
					Mount: mount.Destination,
					RW:    mount.RW,
					Id:    item.ID,
				})
			}
		}
	}
	self.JsonResponseWithoutError(http, gin.H{
		"info":           volumeInfo,
		"inUseContainer": inUseContainer,
	})
	return
}

func (self Volume) Create(http *gin.Context) {
	type ParamsValidate struct {
		Name          string   `json:"name" binding:"required"`
		Driver        string   `json:"driver" binding:"omitempty,oneof=local"`
		Type          string   `json:"type" binding:"omitempty,oneof=default tmpfs nfs nfs4 other"`
		NfsUrl        string   `json:"nfsUrl"`
		NfsMountPoint string   `json:"nfsMountPoint"`
		NfsOptions    string   `json:"nfsOptions"`
		TmpfsOptions  string   `json:"tmpfsOptions"`
		OtherOptions  []string `json:"otherOptions"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	options := make(map[string]string)
	switch params.Type {
	case "tmpfs":
		options["type"] = "tmpfs"
		options["device"] = "tmpfs"
		options["o"] = params.TmpfsOptions
	case "nfs", "nfs4":
		options["type"] = params.Type
		options["device"] = ":" + strings.TrimPrefix(params.NfsMountPoint, ":")
		options["o"] = params.NfsUrl + "," + params.NfsOptions
	case "other":
		for _, row := range params.OtherOptions {
			item := strings.Split(row, "\n")
			options[item[0]] = item[1]
		}
	}
	volumeInfo, err := docker.Sdk.Client.VolumeCreate(docker.Sdk.Ctx, volume.CreateOptions{
		Driver:     params.Driver,
		Name:       params.Name,
		DriverOpts: options,
	})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonResponseWithoutError(http, gin.H{
		"info": volumeInfo,
	})
	return

}

func (self Volume) Prune(http *gin.Context) {
	type ParamsValidate struct {
		DeleteAll bool `json:"deleteAll"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	filter := filters.NewArgs()
	res, err := docker.Sdk.Client.VolumesPrune(docker.Sdk.Ctx, filter)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	// 清理非匿名未使用卷
	if params.DeleteAll {
		volumeList, err := docker.Sdk.Client.VolumeList(docker.Sdk.Ctx, volume.ListOptions{})
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		var unUseVolume []string
		containerList, err := docker.Sdk.Client.ContainerList(docker.Sdk.Ctx, container.ListOptions{
			All:    true,
			Latest: true,
		})
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		for _, item := range volumeList.Volumes {
			has := false
			for _, container := range containerList {
				for _, mount := range container.Mounts {
					if mount.Name != "" && mount.Name == item.Name {
						has = true
					}
				}
			}
			if !has {
				if item.UsageData != nil {
					res.SpaceReclaimed += uint64(item.UsageData.Size)
				}
				unUseVolume = append(unUseVolume, item.Name)
			}
		}

		for _, item := range unUseVolume {
			res.VolumesDeleted = append(res.VolumesDeleted, item)
			err = docker.Sdk.Client.VolumeRemove(docker.Sdk.Ctx, item, false)
			if err != nil {
				self.JsonResponseWithError(http, err, 500)
				return
			}
		}
	}
	_ = notice.Message{}.Info(".volumePrune", "count", fmt.Sprintf("%d", len(res.VolumesDeleted)), "size", units.HumanSize(float64(res.SpaceReclaimed)))
	self.JsonSuccessResponse(http)
	return
}

func (self Volume) Delete(http *gin.Context) {
	type ParamsValidate struct {
		Name []string `json:"name" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	for _, name := range params.Name {
		err := docker.Sdk.Client.VolumeRemove(docker.Sdk.Ctx, name, false)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}
	self.JsonSuccessResponse(http)
	return
}
