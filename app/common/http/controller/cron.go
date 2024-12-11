package controller

import (
	"errors"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/service/crontab"
	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
)

type Cron struct {
	controller.Abstract
}

func (self Cron) Create(http *gin.Context) {
	type ParamsValidate struct {
		Id            int32                            `json:"id"`
		Title         string                           `json:"title" binding:"required"`
		Expression    []accessor.CronSettingExpression `json:"expression" binding:"required"`
		ContainerName string                           `json:"containerName" binding:"required"`
		Script        string                           `json:"script" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	allExpression := make([]string, 0)
	for _, expression := range params.Expression {
		allExpression = append(allExpression, expression.ToString())
	}
	err := crontab.Wrapper.CheckExpression(allExpression)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	var taskRow *entity.Cron
	if params.Id > 0 {
		taskRow, _ = dao.Cron.Where(dao.Cron.ID.Eq(params.Id)).First()
		crontab.Wrapper.RemoveJob(taskRow.Setting.JobIds...)
		taskRow.Setting.Expression = params.Expression
	} else {
		if _, err := dao.Cron.Where(dao.Cron.Title.Like(params.Title)).First(); err == nil {
			self.JsonResponseWithError(http, errors.New("任务名称已经存在"), 500)
			return
		}
		taskRow = &entity.Cron{
			Title: params.Title,
			Setting: &accessor.CronSettingOption{
				NextRunTime:   nil,
				Expression:    params.Expression,
				ContainerName: params.ContainerName,
				Script:        params.Script,
				JobIds:        make([]cron.EntryID, 0),
			},
		}
		err = dao.Cron.Create(taskRow)
	}

	ids, err := crontab.Wrapper.AddJob(allExpression, &crontab.Job{
		Script: params.Script,
		Id:     taskRow.ID,
	})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	taskRow.Setting.JobIds = ids
	taskRow.Setting.NextRunTime = crontab.Wrapper.GetNextRunTime(ids...)
	_, err = dao.Cron.Updates(taskRow)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonSuccessResponse(http)
	return
}

func (self Cron) GetList(http *gin.Context) {
	type ParamsValidate struct {
		Title    string `json:"title"`
		Page     int    `form:"page,default=1" binding:"omitempty,gt=0"`
		PageSize int    `form:"pageSize" binding:"omitempty"`
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
	query := dao.Cron.Order(dao.Cron.ID.Desc())
	if params.Title != "" {
		query.Where(dao.Cron.Title.Like("%" + params.Title + "%"))
	}
	list, total, _ := query.FindByPage((params.Page-1)*params.PageSize, params.PageSize)
	self.JsonResponseWithoutError(http, gin.H{
		"total": total,
		"page":  params.Page,
		"list":  list,
	})
	return
}

func (self Cron) Delete(http *gin.Context) {
	type ParamsValidate struct {
		Id []int32 `json:"id" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	if list, err := dao.Cron.Where(dao.Cron.ID.In(params.Id...)).Find(); err == nil {
		for _, item := range list {
			crontab.Wrapper.RemoveJob(item.Setting.JobIds...)
			_, _ = dao.Cron.Delete(item)
		}
	}
	self.JsonSuccessResponse(http)
	return
}
