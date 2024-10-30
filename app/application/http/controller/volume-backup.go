package controller

import (
	"errors"
	"fmt"
	"github.com/docker/docker/api/types/mount"
	"github.com/donknap/dpanel/app/application/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/plugin"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
	"log/slog"
	"path/filepath"
	"strings"
	"time"
)

var pluginName = plugin.PluginBackup

func (self Volume) GetBackupList(http *gin.Context) {
	type ParamsValidate struct {
		ContainerId string `json:"containerId"`
		Page        int    `json:"page,default=1" binding:"omitempty,gt=0"`
		PageSize    int    `json:"pageSize" binding:"omitempty,gt=1"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 {
		params.PageSize = 10
	}

	query := dao.Backup.Order(dao.Backup.ID.Desc())
	if params.ContainerId != "" {
		query = query.Where(dao.Backup.ContainerID.Like("%" + params.ContainerId + "%"))
	}
	list, total, _ := query.FindByPage((params.Page-1)*params.PageSize, params.PageSize)
	self.JsonResponseWithoutError(http, gin.H{
		"total": total,
		"page":  params.Page,
		"list":  list,
	})
	return
}

func (self Volume) DeleteBackup(http *gin.Context) {
	type ParamsValidate struct {
		Id []int32 `json:"id" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	backupInfo, _ := dao.Backup.Where(dao.Backup.ID.In(params.Id...)).Find()
	var volumeList []string
	var cmdList []string

	for _, item := range backupInfo {
		renameRootPath := fmt.Sprintf("/backup%d", item.ID)
		cmdList = append(cmdList, fmt.Sprintf(`rm -r %s`, strings.Replace(item.Setting.BackupTar, "/backup", renameRootPath, 1)))
		volumeList = append(volumeList, fmt.Sprintf("%s:%s:rw", item.Setting.BackupPath, renameRootPath))
	}
	backupPlugin, err := plugin.NewPlugin(pluginName, map[string]*plugin.TemplateParser{
		pluginName: {
			Command: []string{
				"/bin/sh", "-c", strings.Join(cmdList, " && "),
			},
			Volumes: volumeList,
		},
	})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	_, err = backupPlugin.Create()
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	_, err = dao.Backup.Where(dao.Backup.ID.In(params.Id...)).Delete()
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonSuccessResponse(http)
	return
}

func (self Volume) Backup(http *gin.Context) {
	type ParamsValidate struct {
		ContainerMd5     string `json:"containerMd5" binding:"required"`
		BackupTargetType string `json:"BackupTargetType" binding:"oneof=host dpanel"`
		BackupPath       string `json:"BackupPath"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

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

	composeTemplateParser := map[string]*plugin.TemplateParser{
		pluginName: &plugin.TemplateParser{},
	}
	composeTemplateParser[pluginName].External.VolumesFrom = []string{
		containerInfo.ID,
	}

	backupTar := fmt.Sprintf("/backup/%s/%s.tar.gz", strings.TrimPrefix(containerInfo.Name, "/"), time.Now().Format(function.YmdHis))
	if params.BackupTargetType == "dpanel" {
		// 因为存储挂载到backup目录，保存时需要再添加一级backup目录
		// 保存数据到面板存储时，需要将面板的存储挂载上
		backupTar = "/backup" + backupTar
		dpanelContainerInfo, err := docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, facade.GetConfig().GetString("app.name"))
		if err != nil {
			self.JsonResponseWithError(http, errors.New("您创建的面板容器名称非默认的 dpanel，请重建并通过环境变量 APP_NAME 指定新的名称。"), 500)
			return
		}
		for _, item := range dpanelContainerInfo.Mounts {
			if item.Destination == "/dpanel" {
				switch item.Type {
				case mount.TypeBind:
					composeTemplateParser[pluginName].Volumes = []string{
						item.Source + ":/backup",
					}
					break
				case mount.TypeVolume:
					composeTemplateParser[pluginName].External.Volumes = []string{
						item.Name + ":/backup",
					}
					break
				default:
					self.JsonResponseWithError(http, errors.New("暂不支持存储类型"), 500)
					return
				}
			}
		}
	} else {
		composeTemplateParser[pluginName].Volumes = []string{
			params.BackupPath + ":/backup",
		}
	}

	cmd := fmt.Sprintf(`mkdir -p %s && tar czvf %s %s`, filepath.Dir(backupTar), backupTar, strings.Join(pathList, " "))
	slog.Debug("volume", "backup", cmd)
	composeTemplateParser[pluginName].Command = []string{
		"/bin/sh", "-c", cmd,
	}

	backupPlugin, err := plugin.NewPlugin(pluginName, composeTemplateParser)
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

	composeTemplateParser := map[string]*plugin.TemplateParser{
		pluginName: &plugin.TemplateParser{
			Command: []string{
				"/bin/sh", "-c", cmd,
			},
		},
	}
	composeTemplateParser[pluginName].External.VolumesFrom = []string{
		containerInfo.ID,
	}
	if backupInfo.Setting.BackupTargetType == logic.BackupTypeDPanel {
		if strings.Contains(backupInfo.Setting.BackupPath, "/") {
			composeTemplateParser[pluginName].Volumes = []string{
				backupInfo.Setting.BackupPath + ":/backup",
			}
		} else {
			composeTemplateParser[pluginName].External.Volumes = []string{
				backupInfo.Setting.BackupPath + ":/backup",
			}
		}
	} else {
		composeTemplateParser[pluginName].Volumes = []string{
			backupInfo.Setting.BackupPath + ":/backup",
		}
	}
	backupPlugin, err := plugin.NewPlugin(pluginName, composeTemplateParser)
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
