package controller

import (
	"github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/crontab"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
	"gorm.io/datatypes"
	"gorm.io/gen"
)

type Cron struct {
	controller.Abstract
}

func (self Cron) Create(http *gin.Context) {
	type ParamsValidate struct {
		Id    int32  `json:"id"`
		Title string `json:"title" binding:"required"`
		accessor.CronSettingOption
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	if params.TriggerType != accessor.CronTriggerTypeCron {
		params.Expression = []accessor.CronSettingExpression{
			{
				Unit: "code",
				Code: "@manual",
			},
		}
	}

	err := crontab.Client.CheckExpression(function.PluckArrayWalk(params.Expression, func(item accessor.CronSettingExpression) (string, bool) {
		return item.ToString(), true
	})...)
	if err != nil {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageContainerCronExpressionInCorrect, "message", err.Error()), 500)
		return
	}

	var taskRow *entity.Cron
	if params.Id > 0 {
		taskRow, _ = dao.Cron.Where(dao.Cron.ID.Eq(params.Id)).First()
		if taskRow == nil {
			self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted), 500)
			return
		}
		crontab.Client.RemoveJob(taskRow.Setting.JobIds...)
		taskRow.Setting = &params.CronSettingOption
		taskRow.Title = params.Title
		taskRow.Setting.JobIds = make([]cron.EntryID, 0)
		taskRow.Setting.DockerEnvName = docker.Sdk.Name
	} else {
		if _, err := dao.Cron.Where(dao.Cron.Title.Like(params.Title)).First(); err == nil {
			self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonIdAlreadyExists, "name", params.Title), 500)
			return
		}
		taskRow = &entity.Cron{
			Title:   params.Title,
			Setting: &params.CronSettingOption,
		}
		taskRow.Setting.JobIds = make([]cron.EntryID, 0)
		taskRow.Setting.DockerEnvName = docker.Sdk.Name
		_ = dao.Cron.Create(taskRow)
	}
	// 仅当任务非手动触发的时候，才有暂停和恢复功能
	if taskRow.Setting.TriggerType == accessor.CronTriggerTypeManual || !params.Disable {
		if jobIds, err := (logic.Cron{}).AddCronJob(taskRow); err == nil {
			taskRow.Setting.JobIds = jobIds
		}
	}

	err = dao.Cron.Save(taskRow)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	self.JsonSuccessResponse(http)
	return
}

func (self Cron) GetList(http *gin.Context) {
	type ParamsValidate struct {
		Title string `json:"title"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	query := dao.Cron.Order(dao.Cron.Title.Desc()).Where(gen.Cond(
		datatypes.JSONQuery("setting").Equals(docker.Sdk.Name, "dockerEnvName"),
	)...)
	if params.Title != "" {
		query.Where(dao.Cron.Title.Like("%" + params.Title + "%"))
	}
	list, _ := query.Find()
	self.JsonResponseWithoutError(http, gin.H{
		"list": function.PluckArrayWalk(list, func(item *entity.Cron) (*entity.Cron, bool) {
			if item.Setting.TriggerType == accessor.CronTriggerTypeCron {
				item.Setting.NextRunTime = crontab.Client.GetNextRunTime(item.Setting.JobIds...)
			}
			return item, true
		}),
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
			crontab.Client.RemoveJob(item.Setting.JobIds...)
			_, _ = dao.Cron.Delete(item)
			_, _ = dao.CronLog.Where(dao.CronLog.CronID.Eq(item.ID)).Delete()
		}
	}
	self.JsonSuccessResponse(http)
	return
}

func (self Cron) RunOnce(http *gin.Context) {
	type ParamsValidate struct {
		Id int32 `json:"id" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	cronRow, _ := dao.Cron.Where(dao.Cron.ID.In(params.Id)).First()
	if cronRow == nil {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted), 500)
		return
	}
	if cronRow.Setting.JobIds == nil {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageContainerCronTaskEmpty), 500)
		return
	}

	for _, id := range cronRow.Setting.JobIds {
		crontab.Client.RunById(id)
	}

	self.JsonSuccessResponse(http)
	return
}

func (self Cron) GetDetail(http *gin.Context) {
	type ParamsValidate struct {
		Id int32 `json:"id" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	cronRow, _ := dao.Cron.Where(dao.Cron.ID.In(params.Id)).First()
	if cronRow == nil {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted), 500)
		return
	}
	self.JsonResponseWithoutError(http, gin.H{
		"detail": cronRow,
	})
	return
}

func (self Cron) GetLogList(http *gin.Context) {
	type ParamsValidate struct {
		Id       int32 `json:"id" binding:"required"`
		Page     int   `json:"page" binding:"omitempty,gt=0"`
		PageSize int   `json:"pageSize" binding:"omitempty"`
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
	query := dao.CronLog.Order(dao.CronLog.ID.Desc()).Where(dao.CronLog.CronID.Eq(params.Id))
	list, total, _ := query.FindByPage((params.Page-1)*params.PageSize, params.PageSize)
	self.JsonResponseWithoutError(http, gin.H{
		"total": total,
		"page":  params.Page,
		"list":  list,
	})
}

func (self Cron) PruneLog(http *gin.Context) {
	type ParamsValidate struct {
		Id int32 `json:"id" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	if oldRow, err := dao.CronLog.Where(dao.CronLog.CronID.Eq(params.Id)).Last(); err == nil {
		_, _ = dao.CronLog.Where(dao.CronLog.ID.Lte(oldRow.ID)).Delete()
	}

	db, err := facade.GetDbFactory().Channel("default")
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	db.Exec("vacuum")
	self.JsonSuccessResponse(http)
	return
}

func (self Cron) Template(http *gin.Context) {
	dpanelScriptPath := "/app/script"
	if facade.GetConfig().GetString("app.env") == "debug" {
		dpanelScriptPath = "./docker/script"
	}
	dpanelTemplate, err := logic.CronTemplate{}.Template(dpanelScriptPath)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	template, err := logic.CronTemplate{}.Template(storage.Local{}.GetScriptTemplatePath())
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonResponseWithoutError(http, gin.H{
		"list": append(dpanelTemplate, template...),
	})
	return
}
