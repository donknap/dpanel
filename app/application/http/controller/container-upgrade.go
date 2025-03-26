package controller

import (
	"errors"
	"fmt"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	logic2 "github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/notice"
	"github.com/donknap/dpanel/common/types/event"
	"github.com/gin-gonic/gin"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
	"log/slog"
	"slices"
	"strings"
	"time"
)

func (self Container) Upgrade(http *gin.Context) {
	type ParamsValidate struct {
		Md5       string `json:"md5" binding:"required"`
		ImageTag  string `json:"imageTag"`
		EnableBak bool   `json:"enableBak"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	containerInfo, err := docker.Sdk.ContainerCopyInspect(params.Md5)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	if containerInfo.Name == "/"+facade.GetConfig().GetString("APP_NAME") || containerInfo.Name == "/dpanel" {
		self.JsonResponseWithError(http, errors.New("面板无法升级自身，请通过【系统更新】查看 dpanel 面板升级脚本"), 500)
		return
	}

	bakTime := time.Now().Format(function.YmdHis)

	// 更新容器时可以更改镜像 tag
	if params.ImageTag != "" {
		containerInfo.Image = params.ImageTag
	}

	imageInfo, err := docker.Sdk.Client.ImageInspect(docker.Sdk.Ctx, containerInfo.Config.Image)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	// 如果旧的容器使用的镜像和重新拉取的镜像一致则不升级
	// 多平台下的其它平台镜像推送后，也会提示更新
	// 不一定就是本平台镜像有更新
	// 这里还是选择对齐 digest
	oldContainerImageId := containerInfo.Image
	if containerInfo.Image == imageInfo.ID {
		//self.JsonResponseWithoutError(http, gin.H{
		//	"containerId": containerInfo.ID,
		//})
		//return
	}

	// 成功的创建一个新的容器后再对旧的进停止或是删除操作
	_ = notice.Message{}.Info(".containerCreate", containerInfo.Name)
	newContainerName := fmt.Sprintf("%s-copy-%s", containerInfo.Name, bakTime)

	out, err := docker.Sdk.Client.ContainerCreate(docker.Sdk.Ctx, containerInfo.Config, containerInfo.HostConfig, &network.NetworkingConfig{
		EndpointsConfig: containerInfo.NetworkSettings.Networks,
	}, &v1.Platform{}, newContainerName)
	if err != nil {
		errRemove := docker.Sdk.Client.ContainerRemove(docker.Sdk.Ctx, newContainerName, container.RemoveOptions{})
		self.JsonResponseWithError(http, errors.Join(err, errRemove), 500)
		return
	}

	if containerInfo.State.Running {
		err = docker.Sdk.Client.ContainerStop(docker.Sdk.Ctx, containerInfo.Name, container.StopOptions{})
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		if containerInfo.HostConfig.AutoRemove {
			// 如果是自动删除，则等待容器自动被销毁
			for {
				time.Sleep(time.Second * 1)
				if _, err = docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, containerInfo.Name); err != nil {
					break
				}
			}
		}
	}

	bakContainerName := fmt.Sprintf("%s-bak-%s", containerInfo.Name, bakTime)
	bakImageName := fmt.Sprintf("%s-bak-%s", containerInfo.Config.Image, bakTime)

	// 未备份旧容器，需要先删除，否则名称会冲突
	if params.EnableBak {
		_ = notice.Message{}.Info(".containerBackup", "name", containerInfo.Name)

		if !containerInfo.HostConfig.AutoRemove {
			// 备份旧容器
			err = docker.Sdk.Client.ContainerRename(
				docker.Sdk.Ctx,
				containerInfo.Name,
				bakContainerName,
			)
			if err != nil {
				self.JsonResponseWithError(http, err, 500)
				return
			}
		}

		if oldContainerImageId != imageInfo.ID {
			// 备份旧镜像
			err = docker.Sdk.Client.ImageTag(
				docker.Sdk.Ctx,
				containerInfo.Image,
				bakImageName,
			)
			if err != nil {
				self.JsonResponseWithError(http, err, 500)
				return
			}
		}
	} else {
		_ = notice.Message{}.Info(".containerRemove", containerInfo.Name)

		if !containerInfo.HostConfig.AutoRemove {
			err = docker.Sdk.Client.ContainerRemove(docker.Sdk.Ctx, containerInfo.Name, container.RemoveOptions{})
			if err != nil {
				self.JsonResponseWithError(http, err, 500)
				return
			}
		}
		_, err = docker.Sdk.Client.ImageRemove(docker.Sdk.Ctx, containerInfo.Image, image.RemoveOptions{})
		if err != nil {
			slog.Debug("container upgrade delete image", "error", err.Error())
		}
	}

	err = docker.Sdk.Client.ContainerRename(
		docker.Sdk.Ctx,
		newContainerName,
		containerInfo.Name,
	)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	if siteRow, _ := dao.Site.Where(dao.Site.ContainerInfo.Eq(&accessor.SiteContainerInfoOption{
		ID: params.Md5,
	})).First(); siteRow != nil {
		siteRow.ContainerInfo.ID = out.ID
		_, _ = dao.Site.Updates(siteRow)
	}

	err = docker.Sdk.Client.ContainerStart(docker.Sdk.Ctx, containerInfo.Name, container.StartOptions{})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	newContainerInfo, err := docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, out.ID)
	if err == nil {
		facade.GetEvent().Publish(event.ContainerEditEvent, event.ContainerPayload{
			InspectInfo:    &newContainerInfo,
			OldInspectInfo: &containerInfo,
			Ctx:            http,
		})
	}

	self.JsonResponseWithoutError(http, gin.H{
		"containerId": out.ID,
	})
	return
}

func (self Container) Ignore(http *gin.Context) {
	type ParamsValidate struct {
		Md5     string `json:"md5" binding:"required"`
		ImageId string `json:"imageId"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	checkIgnore := accessor.IgnoreCheckUpgrade{}
	logic2.Setting{}.GetByKey(logic2.SettingGroupSetting, logic2.SettingGroupSettingCheckContainerIgnore, &checkIgnore)

	ignore := fmt.Sprintf("%s@%s", params.Md5, params.ImageId)
	exists, i := function.IndexArrayWalk(checkIgnore, func(i string) bool {
		return strings.HasPrefix(string(i), params.Md5+"@")
	})

	if params.ImageId == "" {
		if exists {
			checkIgnore = slices.Delete(checkIgnore, i, i+1)
		}
	} else {
		if exists {
			checkIgnore[i] = ignore
		} else {
			checkIgnore = append(checkIgnore, ignore)
		}
	}

	_ = logic2.Setting{}.Save(&entity.Setting{
		GroupName: logic2.SettingGroupSetting,
		Name:      logic2.SettingGroupSettingCheckContainerIgnore,
		Value: &accessor.SettingValueOption{
			IgnoreCheckUpgrade: checkIgnore,
		},
	})

	self.JsonSuccessResponse(http)
	return
}
