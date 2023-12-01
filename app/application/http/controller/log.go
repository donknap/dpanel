package controller

import (
	"errors"
	"github.com/donknap/dpanel/app/application/logic"
	"github.com/donknap/dpanel/common/dao"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/src/http/controller"
)

type Log struct {
	controller.Abstract
}

func (self Log) Task(http *gin.Context) {
	type ParamsValidate struct {
		SiteId int32 `form:"siteId" binding:"required,number"`
	}

	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	taskRow, _ := dao.Task.Where(dao.Task.SiteID.Eq(params.SiteId)).First()
	if taskRow == nil {
		self.JsonResponseWithError(http, errors.New("当前没有进行的中任务"), 500)
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
		"status":                   taskRow.Status,
		"step":                     taskRow.Step,
		"message":                  taskRow.Message,
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
	if logic.StepStatusValue[taskRow.Step] == 1 {
		stepStatus[logic.STEP_IMAGE_BUILD] = taskRow.Status
		if taskRow.Status == logic.STATUS_ERROR {
			result[logic.STEP_IMAGE_BUILD] = finishProgress
		}
	}
	if logic.StepStatusValue[taskRow.Step] > 1 {
		stepStatus[logic.STEP_IMAGE_BUILD] = logic.STATUS_SUCCESS
		result[logic.STEP_IMAGE_BUILD] = finishProgress
	}
	// 拉取镜像状态
	if logic.StepStatusValue[taskRow.Step] == 2 {
		stepStatus[logic.STEP_IMAGE_PULL] = taskRow.Status
		if taskRow.Status == logic.STATUS_ERROR {
			result[logic.STEP_IMAGE_PULL] = finishProgress
		}
	}
	if logic.StepStatusValue[taskRow.Step] > 2 {
		stepStatus[logic.STEP_IMAGE_PULL] = logic.STATUS_SUCCESS
		result[logic.STEP_IMAGE_PULL] = finishProgress
	}
	// 构建容器状态
	if logic.StepStatusValue[taskRow.Step] == 3 {
		stepStatus[logic.STEP_CONTAINER_BUILD] = taskRow.Status
		if taskRow.Status == logic.STATUS_ERROR {
			result[logic.STEP_CONTAINER_BUILD] = finishProgress
		}
	}
	if logic.StepStatusValue[taskRow.Step] > 3 {
		stepStatus[logic.STEP_CONTAINER_BUILD] = logic.STATUS_SUCCESS
		result[logic.STEP_CONTAINER_BUILD] = finishProgress
	}
	// 运行容器状态
	if logic.StepStatusValue[taskRow.Step] == 4 {
		stepStatus[logic.STEP_CONTAINER_RUN] = taskRow.Status
		result[logic.STEP_CONTAINER_RUN] = finishProgress
	}
	result["stepStatus"] = stepStatus

	// 只有在拉取镜像时，才获取拉取进度
	if logic.StepStatusValue[taskRow.Step] == 2 {
		task := logic.NewContainerTask()
		stepLog := task.GetTaskStepLog(taskRow.SiteID)
		if stepLog != nil {
			result[logic.STEP_IMAGE_PULL] = stepLog.GetProcess()
			if result[logic.STEP_IMAGE_PULL] == nil {
				if taskRow.Status == logic.STATUS_PROCESSING {
					result[logic.STEP_IMAGE_PULL] = defaultProgress
				} else {
					result[logic.STEP_IMAGE_PULL] = finishProgress
				}
			}
		}
	}

	self.JsonResponseWithoutError(http, result)
	return
}

func (self Log) Run(http *gin.Context) {

}
