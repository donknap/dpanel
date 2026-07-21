package controller

import (
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/donknap/dpanel/app/application/logic"
	logic2 "github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	builder "github.com/donknap/dpanel/common/service/docker/container"
	dockerTypes "github.com/donknap/dpanel/common/service/docker/types"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/service/ws"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/donknap/dpanel/common/types/event"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
	"gorm.io/datatypes"
	"gorm.io/gen"
)

type ContainerUpgrade struct {
	controller.Abstract
}

func (self ContainerUpgrade) Upgrade(http *gin.Context) {
	type ParamsValidate struct {
		Md5                    string `json:"md5" binding:"required"`
		ImageTag               string `json:"imageTag"`
		EnableBak              bool   `json:"enableBak"`
		EnableResetImageConfig bool   `json:"enableResetImageConfig"` // 重置镜像内的配置
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	containerInfo, err := docker.Sdk.ContainerCopyInspect(docker.Sdk.Ctx, params.Md5)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	if containerInfo.Name == "/"+facade.GetConfig().GetString("APP_NAME") {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageContainerUpgradeDPanel), 500)
		return
	}
	startContainer := containerInfo.State.Running
	progressSteps := []string{define.ContainerUpgradeStepCreate}
	if startContainer {
		progressSteps = append(progressSteps, define.ContainerUpgradeStepStop)
	}
	progressSteps = append(progressSteps, define.ContainerUpgradeStepReplace)
	if startContainer {
		progressSteps = append(progressSteps, define.ContainerUpgradeStepStart)
	}
	progress := ws.NewProgressPip(fmt.Sprintf(ws.MessageTypeContainerUpgrade, containerInfo.ID))
	defer progress.Close()
	progressCurrent := 0
	progress.BroadcastMessage(logic.ContainerUpgradeProgress{
		Steps:   progressSteps,
		Current: progressCurrent,
		Total:   len(progressSteps),
	})

	progressCurrent++
	progress.BroadcastMessage(logic.ContainerUpgradeProgress{
		Steps:   progressSteps,
		Current: progressCurrent,
		Total:   len(progressSteps),
	})

	bakTime := time.Now().Format(define.DateYmdHis)

	// 更新容器时可以更改镜像 tag
	imageName := containerInfo.Config.Image
	if params.ImageTag != "" {
		imageName = params.ImageTag
	}

	imageInfo, err := docker.Sdk.Client.ImageInspect(docker.Sdk.Ctx, imageName)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	// 如果旧的容器使用的镜像和重新拉取的镜像一致则不升级
	// 多平台下的其它平台镜像推送后，也会导致 digest 不一致
	// 不一定就是本平台镜像有更新
	// 这里还是选择更新对齐 digest
	oldContainerImageId := containerInfo.Image
	if containerInfo.Image == imageInfo.ID {
		//self.JsonResponseWithoutError(http, gin.H{
		//	"containerId": containerInfo.ID,
		//})
		//return
	}

	// 成功的创建一个新的容器后再对旧的进停止或是删除操作
	newContainerName := fmt.Sprintf("%s-copy-%s", containerInfo.Name, bakTime)

	options := []builder.Option{
		builder.WithContainerInfo(containerInfo),
		builder.WithContainerName(newContainerName),
		builder.WithImage(imageName, false),
	}
	if containerInfo.NetworkSettings != nil {
		for name, endpoint := range containerInfo.NetworkSettings.Networks {
			options = append(options, builder.WithNetworkEndpoint(name, endpoint))
		}
	}
	if params.EnableResetImageConfig {
		options = append(options,
			builder.WithEnv(function.PluckArrayWalk(imageInfo.Config.Env, func(item string) (dockerTypes.EnvItem, bool) {
				return dockerTypes.NewEnvItemFromString(item), true
			})...),
			builder.WithLabels(function.PluckMapWalkArray(imageInfo.Config.Labels, func(name string, value string) (dockerTypes.ValueItem, bool) {
				return dockerTypes.ValueItem{
					Name:  name,
					Value: value,
				}, true
			})...),
			builder.WithWorkDir(imageInfo.Config.WorkingDir),
			builder.WithCommand(imageInfo.Config.Cmd),
			builder.WithEntrypoint(imageInfo.Config.Entrypoint),
		)
	}
	containerBuilder, err := builder.New(options...)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	out, err := containerBuilder.Execute()
	if err != nil {
		errRemove := docker.Sdk.Client.ContainerRemove(docker.Sdk.Ctx, newContainerName, container.RemoveOptions{})
		self.JsonResponseWithError(http, errors.Join(err, errRemove), 500)
		return
	}

	if containerInfo.State.Running {
		progressCurrent++
		progress.BroadcastMessage(logic.ContainerUpgradeProgress{
			Steps:   progressSteps,
			Current: progressCurrent,
			Total:   len(progressSteps),
		})
		err = docker.Sdk.Client.ContainerStop(docker.Sdk.Ctx, containerInfo.Name, container.StopOptions{})
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		if containerInfo.HostConfig.AutoRemove {
			// 如果是旧容器配置了自动删除，则等待容器自动被销毁
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
	progressCurrent++
	progress.BroadcastMessage(logic.ContainerUpgradeProgress{
		Steps:   progressSteps,
		Current: progressCurrent,
		Total:   len(progressSteps),
	})

	// 未备份旧容器，需要先删除，否则名称会冲突
	if params.EnableBak {
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

	newContainerInfo, err := docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, out.ID)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	// 容器升级后，将表中的数据更新为新的容器数据
	if siteRow, _ := dao.Site.Where(gen.Cond(datatypes.JSONQuery("container_info").Equals(params.Md5, "Id"))...).First(); siteRow != nil {
		siteRow.ContainerInfo = &accessor.SiteContainerInfoOption{
			Id:   out.ID,
			Info: newContainerInfo,
		}
		_ = dao.Site.Save(siteRow)
	}

	// 旧容器如果是停止状态，重建后保持不启动
	if startContainer {
		progressCurrent++
		progress.BroadcastMessage(logic.ContainerUpgradeProgress{
			Steps:   progressSteps,
			Current: progressCurrent,
			Total:   len(progressSteps),
		})
		err = docker.Sdk.Client.ContainerStart(docker.Sdk.Ctx, containerInfo.Name, container.StartOptions{})
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}

	facade.GetEvent().Publish(event.ContainerEditEvent, event.ContainerPayload{
		InspectInfo:    &newContainerInfo,
		OldInspectInfo: &containerInfo,
		Ctx:            http,
	})

	self.JsonResponseWithoutError(http, gin.H{
		"containerId": out.ID,
	})
	return
}

func (self ContainerUpgrade) Ignore(http *gin.Context) {
	type ParamsValidate struct {
		Md5     string `json:"md5" binding:"required"`
		ImageId string `json:"imageId"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	checkIgnore := accessor.ContainerCheckIgnoreUpgrade{}
	logic2.Setting{}.GetByKey(logic2.SettingGroupSetting, logic2.SettingGroupSettingCheckContainerIgnore, &checkIgnore)

	ignore := fmt.Sprintf("%s@%s", params.Md5, params.ImageId)
	i, ok := function.IndexArrayWalk(checkIgnore, func(i string) bool {
		return strings.HasPrefix(i, params.Md5+"@")
	})

	if params.ImageId == "" {
		if ok {
			checkIgnore = slices.Delete(checkIgnore, i, i+1)
		}
	} else {
		if ok {
			checkIgnore[i] = ignore
		} else {
			checkIgnore = append(checkIgnore, ignore)
		}
	}

	_ = logic2.Setting{}.Save(&entity.Setting{
		GroupName: logic2.SettingGroupSetting,
		Name:      logic2.SettingGroupSettingCheckContainerIgnore,
		Value: &accessor.SettingValueOption{
			ContainerCheckIgnoreUpgrade: checkIgnore,
		},
	})

	self.JsonSuccessResponse(http)
	return
}

func (self ContainerUpgrade) Check(http *gin.Context) {
	type ParamsValidate struct {
		ContainerID string `json:"containerId" binding:"required"`
		Force       bool   `json:"force"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	params.ContainerID = strings.TrimSpace(params.ContainerID)
	if params.ContainerID == "" {
		self.JsonResponseWithError(http, errors.New("containerId is required"), 500)
		return
	}

	dockerSdk, err := docker.NewClientWithUser(http)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	containerInfo, err := dockerSdk.Client.ContainerInspect(dockerSdk.Ctx, params.ContainerID)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	ignore := accessor.ContainerCheckIgnoreUpgrade{}
	logic2.Setting{}.GetByKey(logic2.SettingGroupSetting, logic2.SettingGroupSettingCheckContainerIgnore, &ignore)
	ignoreStatus := ""
	if function.InArray(ignore, fmt.Sprintf("%s@*", containerInfo.Name)) {
		ignoreStatus = define.ContainerUpgradeStatusIgnoreAlways
	} else if function.InArray(ignore, fmt.Sprintf("%s@%s", containerInfo.Name, containerInfo.Image)) {
		ignoreStatus = define.ContainerUpgradeStatusIgnoreCurrent
	}
	if ignoreStatus != "" {
		self.JsonResponseWithoutError(http, gin.H{
			"upgrade":     false,
			"digest":      "",
			"digestLocal": make([]string, 0),
			"error":       "",
			"status":      ignoreStatus,
		})
		return
	}

	result, err := (logic.ContainerUpgrade{}).Check(dockerSdk, &containerInfo, params.Force)
	errorMessage := ""
	if err != nil {
		errorMessage = err.Error()
	}
	self.JsonResponseWithoutError(http, gin.H{
		"upgrade":     result.Status == define.ContainerUpgradeStatusUpgrade,
		"digest":      result.RemoteDigest,
		"digestLocal": result.LocalDigest,
		"error":       errorMessage,
		"status":      result.Status,
	})
}

func (self ContainerUpgrade) GetList(http *gin.Context) {
	dockerSdk, err := docker.NewClientWithUser(http)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	ignore := accessor.ContainerCheckIgnoreUpgrade{}
	logic2.Setting{}.GetByKey(logic2.SettingGroupSetting, logic2.SettingGroupSettingCheckContainerIgnore, &ignore)
	if ignore == nil {
		ignore = make(accessor.ContainerCheckIgnoreUpgrade, 0)
	}

	containerList, err := dockerSdk.Client.ContainerList(dockerSdk.Ctx, container.ListOptions{All: true})
	if http.Request.Context().Err() != nil {
		return
	}
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	list := make([]gin.H, 0, len(containerList))
	for _, item := range containerList {
		if hidden, ok := item.Labels[define.DPanelLabelContainerHidden]; ok && (hidden == "true" || hidden == "1") {
			continue
		}

		containerName := ""
		if len(item.Names) > 0 {
			containerName = item.Names[0]
		}
		checkedAt := ""
		errorMessage := ""
		status := define.ContainerUpgradeStatusUnchecked
		cacheKey := fmt.Sprintf(storage.CacheKeyContainerUpgrade, dockerSdk.Name, item.ID)
		value, exists := storage.Cache.Get(cacheKey)
		if exists {
			cached, ok := value.(logic.ContainerUpgradeResult)
			if !ok {
				storage.Cache.Delete(cacheKey)
			} else {
				checkedAt = cached.CheckedAt
				// 容器镜像如果已经更新，这里的名称会变成 sha256 的形式，但是返回的值必须是镜像的原始名称
				item.Image = cached.ImageName
				switch cached.Status {
				case define.ContainerUpgradeStatusFailed,
					define.ContainerUpgradeStatusLatest,
					define.ContainerUpgradeStatusUnavailable,
					define.ContainerUpgradeStatusUpgrade:
					status = cached.Status
					if cached.Error != nil {
						errorMessage = cached.Error.Error()
					}
				default:
					storage.Cache.Delete(cacheKey)
				}
			}
		}
		if function.InArray(ignore, fmt.Sprintf("%s@*", containerName)) {
			status = define.ContainerUpgradeStatusIgnoreAlways
			errorMessage = ""
		} else if function.InArray(ignore, fmt.Sprintf("%s@%s", containerName, item.ImageID)) {
			status = define.ContainerUpgradeStatusIgnoreCurrent
			errorMessage = ""
		}
		list = append(list, gin.H{
			"checkedAt":     checkedAt,
			"containerId":   item.ID,
			"containerName": containerName,
			"error":         errorMessage,
			"imageId":       item.ImageID,
			"imageName":     item.Image,
			"status":        status,
		})
	}

	sort.Slice(list, func(i, j int) bool {
		if list[i]["containerName"] == list[j]["containerName"] {
			return list[i]["containerId"].(string) < list[j]["containerId"].(string)
		}
		return list[i]["containerName"].(string) < list[j]["containerName"].(string)
	})
	self.JsonResponseWithoutError(http, gin.H{
		"list": list,
	})
}
