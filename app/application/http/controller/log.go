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
	if taskRow.Status != logic.STATUS_PROCESSING {
		self.JsonResponseWithoutError(http, gin.H{
			"status":     taskRow.Status,
			"step":       taskRow.Step,
			"stepStatus": logic.StepStatus[taskRow.Step],
			"message":    taskRow.Message,
		})
		return
	}
	task := logic.NewContainerTask()
	stepLog := task.GetTaskStepLog(taskRow.SiteID)
	if stepLog == nil {
		self.JsonResponseWithError(http, errors.New("当前没有进行的中任务或是已经完成"), 500)
		return
	}
	self.JsonResponseWithoutError(
		http, gin.H{
			"status":              taskRow.Status,
			"step":                taskRow.Step,
			"stepStatus":          logic.StepStatus[taskRow.Step],
			logic.STEP_IMAGE_PULL: stepLog.GetProcess(),
		},
	)
	return
}

func (self Log) Run(http *gin.Context) {

}
