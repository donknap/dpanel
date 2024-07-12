package controller

import (
	"errors"
	"fmt"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/volume"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/plugin"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/src/http/controller"
	"log/slog"
	"path/filepath"
	"strings"
	"time"
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
	var inUseVolume []string
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
				inUseVolume = append(inUseVolume, mount.Name)
			}
		}
	}
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
	filter := filters.NewArgs()
	_, err := docker.Sdk.Client.VolumesPrune(docker.Sdk.Ctx, filter)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
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

func (self Volume) Backup(http *gin.Context) {
	type ParamsValidate struct {
		ContainerMd5     string `json:"containerMd5"`
		BackupTargetType string `json:"BackupTargetType" binding:"oneof=host dpanel"`
		BackupPath       string `json:"BackupPath"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	pluginName := "backup"

	if params.ContainerMd5 != "" {
		containerInfo, err := docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, params.ContainerMd5)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		var pathList []string
		for _, mount := range containerInfo.Mounts {
			pathList = append(pathList, mount.Destination)
		}
		if function.IsEmptyArray(pathList) {
			self.JsonResponseWithError(http, errors.New("该容器没有绑定存储，请直接导出容器"), 500)
			return
		}
		backupTar := fmt.Sprintf("/backup/%s/%s.tar.gz", strings.TrimPrefix(containerInfo.Name, "/"), time.Now().Format(function.YmdHis))
		if params.BackupTargetType == "dpanel" {
			// 因为存储挂载到backup目录，保存时需要再添加一级backup目录
			backupTar = "/backup" + backupTar
		}
		cmd := fmt.Sprintf(`mkdir -p %s && tar czvf %s %s`, filepath.Dir(backupTar), backupTar, strings.Join(pathList, " "))
		slog.Debug("volume", "backup", cmd)

		backupPlugin, err := plugin.NewPlugin(pluginName, map[string]docker.ComposeService{
			pluginName: {
				Command: []string{
					"/bin/sh", "-c", cmd,
				},
				VolumesFrom: []string{
					containerInfo.ID,
				},
				Volumes: []string{
					params.BackupPath + ":/backup:rw",
				},
			},
		})
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		_, err = backupPlugin.Create()
		if err != nil {
			_ = backupPlugin.Destroy()
			self.JsonResponseWithError(http, err, 500)
			return
		}
		err = dao.Backup.Create(&entity.Backup{
			ContainerID: containerInfo.Name,
			Setting: &accessor.BackupSettingOption{
				BackupTar:        backupTar,
				BackupTargetType: params.BackupTargetType,
				BackupPath:       params.BackupPath,
				VolumePathList:   pathList,
			},
		})
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}

		self.JsonResponseWithoutError(http, gin.H{
			"backupTar":        backupTar,
			"pathList":         pathList,
			"backupTargetType": params.BackupTargetType,
		})
		return
	}
	return
}

func (self Volume) Restore(http *gin.Context) {
	type ParamsValidate struct {
		Id           int32  `json:"id" binding:"required"`
		ContainerMd5 string `json:"containerMd5" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	backupInfo, _ := dao.Backup.Where(dao.Backup.ID.Eq(params.Id)).First()
	if backupInfo == nil {
		self.JsonResponseWithError(http, errors.New("备份数据不存在"), 500)
		return
	}
	containerInfo, err := docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, params.ContainerMd5)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	cmd := fmt.Sprintf(`tar xzvf %s`, backupInfo.Setting.BackupTar)
	slog.Debug("volume", "restore", cmd)

	pluginName := "backup"
	backupPlugin, err := plugin.NewPlugin(pluginName, map[string]docker.ComposeService{
		pluginName: {
			Command: []string{
				"/bin/sh", "-c", cmd,
			},
			VolumesFrom: []string{
				containerInfo.ID,
			},
			Volumes: []string{
				backupInfo.Setting.BackupPath + ":/backup:rw",
			},
		},
	})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	_, err = backupPlugin.Create()
	if err != nil {
		_ = backupPlugin.Destroy()
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonSuccessResponse(http)
	return
}
