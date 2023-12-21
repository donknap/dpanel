package controller

import (
	"errors"
	"github.com/donknap/dpanel/app/application/logic"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/src/http/controller"
)

type RunLog struct {
	controller.Abstract
}

func (self RunLog) Task(http *gin.Context) {
	type ParamsValidate struct {
		Id int32 `form:"id" binding:"required,number"`
	}

	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	siteRow, _ := dao.Site.Where(dao.Site.ID.Eq(params.Id)).Last()
	if siteRow == nil {
		self.JsonResponseWithError(http, errors.New("当前站点不存在"), 500)
		return
	}

	defaultProgress := map[string]struct {
		Downloading float64 `json:"downloading"`
		Extracting  float64 `json:"extracting"`
	}{
		"default": {Downloading: 0, Extracting: 0},
	}

	finishProgress := map[string]struct {
		Downloading float64 `json:"downloading"`
		Extracting  float64 `json:"extracting"`
	}{
		"default": {Downloading: 100, Extracting: 100},
	}

	result := gin.H{
		"status":                   siteRow.Status,
		"step":                     siteRow.StatusStep,
		"message":                  siteRow.Message,
		logic.STEP_IMAGE_BUILD:     defaultProgress,
		logic.STEP_IMAGE_PULL:      defaultProgress,
		logic.STEP_CONTAINER_BUILD: defaultProgress,
		logic.STEP_CONTAINER_RUN:   defaultProgress,
	}

	stepStatus := map[string]int32{
		logic.STEP_IMAGE_BUILD:     0,
		logic.STEP_IMAGE_PULL:      0,
		logic.STEP_CONTAINER_BUILD: 0,
		logic.STEP_CONTAINER_RUN:   0,
	}
	// 构建镜像状态
	if logic.StepStatusValue[siteRow.StatusStep] == 1 {
		stepStatus[logic.STEP_IMAGE_BUILD] = siteRow.Status
		if siteRow.Status == logic.STATUS_ERROR {
			result[logic.STEP_IMAGE_BUILD] = finishProgress
		}
	}
	if logic.StepStatusValue[siteRow.StatusStep] > 1 {
		stepStatus[logic.STEP_IMAGE_BUILD] = logic.STATUS_SUCCESS
		result[logic.STEP_IMAGE_BUILD] = finishProgress
	}
	// 拉取镜像状态
	if logic.StepStatusValue[siteRow.StatusStep] == 2 {
		stepStatus[logic.STEP_IMAGE_PULL] = siteRow.Status
		if siteRow.Status == logic.STATUS_ERROR {
			result[logic.STEP_IMAGE_PULL] = finishProgress
		}
	}
	if logic.StepStatusValue[siteRow.StatusStep] > 2 {
		stepStatus[logic.STEP_IMAGE_PULL] = logic.STATUS_SUCCESS
		result[logic.STEP_IMAGE_PULL] = finishProgress
	}
	// 构建容器状态
	if logic.StepStatusValue[siteRow.StatusStep] == 3 {
		stepStatus[logic.STEP_CONTAINER_BUILD] = siteRow.Status
		if siteRow.Status == logic.STATUS_ERROR {
			result[logic.STEP_CONTAINER_BUILD] = finishProgress
		}
	}
	if logic.StepStatusValue[siteRow.StatusStep] > 3 {
		stepStatus[logic.STEP_CONTAINER_BUILD] = logic.STATUS_SUCCESS
		result[logic.STEP_CONTAINER_BUILD] = finishProgress
	}
	// 运行容器状态
	if logic.StepStatusValue[siteRow.StatusStep] == 4 {
		stepStatus[logic.STEP_CONTAINER_RUN] = siteRow.Status
		result[logic.STEP_CONTAINER_RUN] = finishProgress
	}
	result["stepStatus"] = stepStatus

	// 只有在拉取镜像时，才获取拉取进度
	if logic.StepStatusValue[siteRow.StatusStep] == 2 {
		//task := logic.NewDockerTask()
		//stepLog := task.GetTaskContainerStepLog(siteRow.ID)
		//if stepLog != nil {
		//	result[logic.STEP_IMAGE_PULL] = stepLog.GetProcess()
		//	if result[logic.STEP_IMAGE_PULL] == nil {
		//		if siteRow.Status == logic.STATUS_PROCESSING {
		//			result[logic.STEP_IMAGE_PULL] = defaultProgress
		//		} else {
		//			result[logic.STEP_IMAGE_PULL] = finishProgress
		//		}
		//	}
		//}
	}
	self.JsonResponseWithoutError(http, result)
	return
}

func (self RunLog) Run(http *gin.Context) {
	type ParamsValidate struct {
		Id        int32 `form:"id" binding:"required,number"`
		LineTotal int   `form:"lineTotal" binding:"required,number,oneof=50 100 200 500 1000"`
	}

	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	siteRow, _ := dao.Site.Where(dao.Site.ID.Eq(params.Id)).First()
	if siteRow == nil {
		self.JsonResponseWithError(http, errors.New("站点不存在"), 500)
		return
	}
	if siteRow.ContainerInfo == nil {
		self.JsonResponseWithError(http, errors.New("当前站点并没有部署成功"), 500)
		return
	}

	builder := docker.Sdk.GetContainerLogBuilder()
	builder.WithContainerId(siteRow.ContainerInfo.Info.ID)
	builder.WithTail(params.LineTotal)
	content, err := builder.Execute()
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonResponseWithoutError(http, gin.H{
		"log": content,
	})
	return
}
