package controller

import (
	"errors"
	"fmt"
	"time"

	"slices"
	"sort"
	"strings"

	"github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/donknap/dpanel/app/application/logic"
	logic2 "github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	builder "github.com/donknap/dpanel/common/service/docker/container"
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

type containerUpgradeProgress struct {
	Steps   []string `json:"steps"`
	Current int      `json:"current"`
	Total   int      `json:"total"`
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
	dockerClient, err := docker.NewClientWithUser(http)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	containerInfo, err := dockerClient.ContainerCopyInspect(dockerClient.Ctx, params.Md5)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	if containerInfo.Name == "/"+facade.GetConfig().GetString("APP_NAME") {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageContainerUpgradeDPanel), 500)
		return
	}
	upgradeMutex := storage.NewMutex(fmt.Sprintf(
		storage.CacheKeyContainerUpgradeRunning,
		dockerClient.Name,
		strings.TrimPrefix(strings.TrimSpace(containerInfo.Name), "/"),
	))
	if !upgradeMutex.TryLock() {
		self.JsonResponseWithError(http, errors.New("container upgrade is running"), 500)
		return
	}
	defer upgradeMutex.Unlock()
	startContainer := containerInfo.State != nil && containerInfo.State.Running
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
	notifyProgress := func() {
		progress.BroadcastMessage(containerUpgradeProgress{
			Steps:   progressSteps,
			Current: progressCurrent,
			Total:   len(progressSteps),
		})
	}
	notifyProgress()

	if containerInfo.Config == nil || containerInfo.HostConfig == nil {
		self.JsonResponseWithError(http, errors.New("container inspect info is incomplete"), 500)
		return
	}
	imageName := params.ImageTag
	if imageName == "" {
		imageName = containerInfo.Config.Image
	}
	options := []builder.Option{
		builder.WithContainerName(containerInfo.Name),
		builder.WithContainerRollback(containerInfo, params.EnableBak),
		builder.WithContainerInfo(containerInfo),
		builder.WithImage(imageName),
		builder.WithImageRuntimeConfig(params.EnableResetImageConfig),
	}
	if containerInfo.NetworkSettings != nil {
		for name, endpoint := range containerInfo.NetworkSettings.Networks {
			options = append(options, builder.WithNetworkEndpoint(name, endpoint))
		}
	}
	containerBuilder, err := builder.New(dockerClient, options...)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	progressCurrent++
	notifyProgress()

	if startContainer {
		progressCurrent++
		notifyProgress()
		if err = dockerClient.Client.ContainerStop(dockerClient.Ctx, containerInfo.ID, container.StopOptions{}); err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		if containerInfo.HostConfig.AutoRemove {
			for {
				_, inspectErr := dockerClient.Client.ContainerInspect(dockerClient.Ctx, containerInfo.ID)
				if errdefs.IsNotFound(inspectErr) {
					break
				}
				if inspectErr != nil {
					self.JsonResponseWithError(http, inspectErr, 500)
					return
				}
				select {
				case <-dockerClient.Ctx.Done():
					self.JsonResponseWithError(http, dockerClient.Ctx.Err(), 500)
					return
				case <-time.After(time.Second):
				}
			}
		}
	}

	progressCurrent++
	notifyProgress()
	containerID, executeErr := containerBuilder.Execute()
	if containerID == "" {
		self.JsonResponseWithError(http, errors.Join(executeErr, errors.New("final container id is empty")), 500)
		return
	}
	newContainerInfo, err := dockerClient.Client.ContainerInspect(dockerClient.Ctx, containerID)
	if err != nil {
		self.JsonResponseWithError(http, errors.Join(executeErr, err), 500)
		return
	}
	if newContainerInfo.ID != containerInfo.ID {
		// 容器 ID 已经变化时，即使后续启动或数据保存失败，也必须同步依赖旧 ID 的权限。
		defer func() {
			facade.GetEvent().Publish(event.ContainerEditEvent, event.ContainerPayload{
				InspectInfo:    &newContainerInfo,
				OldInspectInfo: &containerInfo,
				Ctx:            http,
			})
		}()
	}
	siteRow, _ := dao.Site.Where(gen.Cond(datatypes.JSONQuery("container_info").Equals(containerInfo.ID, "Id"))...).First()
	if siteRow != nil {
		siteRow.ContainerInfo = &accessor.SiteContainerInfoOption{Id: newContainerInfo.ID, Info: newContainerInfo}
		if err = dao.Site.Save(siteRow); err != nil {
			self.JsonResponseWithError(http, errors.Join(executeErr, err), 500)
			return
		}
	}
	if executeErr != nil {
		self.JsonResponseWithError(http, executeErr, 500)
		return
	}
	if startContainer {
		progressCurrent++
		notifyProgress()
		if err = dockerClient.Client.ContainerStart(dockerClient.Ctx, containerID, container.StartOptions{}); err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		newContainerInfo, err = dockerClient.Client.ContainerInspect(dockerClient.Ctx, containerID)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		if siteRow != nil {
			siteRow.ContainerInfo = &accessor.SiteContainerInfoOption{Id: newContainerInfo.ID, Info: newContainerInfo}
			if err = dao.Site.Save(siteRow); err != nil {
				self.JsonResponseWithError(http, err, 500)
				return
			}
		}
	}

	self.JsonResponseWithoutError(http, gin.H{
		"containerId": newContainerInfo.ID,
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

	result := (logic.Container{}).CheckUpgrade(dockerSdk, containerInfo, params.Force)
	self.JsonResponseWithoutError(http, gin.H{
		"upgrade":     result.Status == define.ContainerUpgradeStatusUpgrade,
		"digest":      result.Result.RemoteDigest,
		"digestLocal": result.Result.LocalDigests,
		"error":       result.Error,
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
		if result, ok := storage.LoadCache[logic.ContainerUpgradeCheckResult](cacheKey); ok {
			checkedAt = result.CheckedAt
			// 容器镜像如果已经更新，这里的名称会变成 sha256 的形式，但是返回的值必须是镜像的原始名称
			item.Image = result.Result.ImageName
			errorMessage = result.Error
			status = result.Status
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
