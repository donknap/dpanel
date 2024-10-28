package controller

import (
	"errors"
	"github.com/donknap/dpanel/app/application/logic"
	logic2 "github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/notice"
	"github.com/gin-gonic/gin"
)

func (self Compose) ContainerDeploy(http *gin.Context) {
	type ParamsValidate struct {
		Id          int32                         `json:"id" binding:"required"`
		Environment map[string][]accessor.EnvItem `json:"environment"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	composeRow, _ := dao.Compose.Where(dao.Compose.ID.Eq(params.Id)).First()
	if composeRow == nil {
		self.JsonResponseWithError(http, errors.New("任务不存在"), 500)
		return
	}
	if params.Environment != nil {
		// 添加当前自定义环境变量到当前docker环境中
		if docker.Sdk.Host != "" {
			dockerEnv, err := logic2.DockerEnv{}.GetEnvByName(docker.Sdk.Host)
			if err != nil {
				self.JsonResponseWithError(http, errors.New("未找到当前docker环境配置，请先添加docker客户端"), 500)
				return
			}
			dockerEnv.Environment = params.Environment
			logic2.DockerEnv{}.UpdateEnv(dockerEnv)
		}
		if composeRow.Setting.Override == nil {
			composeRow.Setting.Override = make(map[string]accessor.SiteEnvOption)
		}
		for name, item := range params.Environment {
			if override, ok := composeRow.Setting.Override[name]; ok {
				override.Environment = item
				composeRow.Setting.Override[name] = override
			} else {
				override = accessor.SiteEnvOption{
					Environment: item,
				}
				composeRow.Setting.Override[name] = override
			}
		}
	}
	tasker, err := logic.Compose{}.GetTasker(composeRow)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	err = tasker.Deploy()
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	// 部署完成后也上更新状态
	composeRun, err := logic.Compose{}.LsItem(tasker.Name)
	if err != nil {
		composeRow.Setting.Status = logic.ComposeStatusWaiting
	} else {
		composeRow.Setting.Status = composeRun.Status
	}
	_, _ = dao.Compose.Updates(composeRow)
	self.JsonSuccessResponse(http)
	return
}

func (self Compose) ContainerDestroy(http *gin.Context) {
	type ParamsValidate struct {
		Id           int32 `json:"id" binding:"required"`
		DeleteImage  bool  `json:"deleteImage"`
		DeleteData   bool  `json:"deleteData"`
		DeleteVolume bool  `json:"deleteVolume"`
	}

	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	composeRow, _ := dao.Compose.Where(dao.Compose.ID.Eq(params.Id)).First()
	if composeRow == nil {
		self.JsonResponseWithError(http, errors.New("任务不存在"), 500)
		return
	}
	tasker, err := logic.Compose{}.GetTasker(composeRow)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	err = tasker.Destroy(params.DeleteImage, params.DeleteVolume)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	if params.DeleteData {
		_, err := dao.Compose.Where(dao.Compose.ID.In(params.Id)).Delete()
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}
	composeRun, err := logic.Compose{}.LsItem(tasker.Name)
	if err != nil {
		composeRow.Setting.Status = logic.ComposeStatusWaiting
	} else {
		composeRow.Setting.Status = composeRun.Status
	}
	_, _ = dao.Compose.Updates(composeRow)
	notice.Message{}.Success("composeDestroy", composeRow.Name)
	self.JsonSuccessResponse(http)
	return
}

func (self Compose) ContainerCtrl(http *gin.Context) {
	type ParamsValidate struct {
		Id int32  `json:"id" binding:"required"`
		Op string `json:"op" binding:"required" oneof:"start restart stop pause unpause ls"`
	}

	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	composeRow, _ := dao.Compose.Where(dao.Compose.ID.Eq(params.Id)).First()
	if composeRow == nil {
		self.JsonResponseWithError(http, errors.New("任务不存在"), 500)
		return
	}
	tasker, err := logic.Compose{}.GetTasker(composeRow)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	err = tasker.Ctrl(params.Op)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	composeRun, err := logic.Compose{}.LsItem(tasker.Name)
	if err != nil {
		composeRow.Setting.Status = logic.ComposeStatusWaiting
	} else {
		composeRow.Setting.Status = composeRun.Status
	}
	_, _ = dao.Compose.Updates(composeRow)
	self.JsonSuccessResponse(http)
	return
}

func (self Compose) ContainerProcessKill(http *gin.Context) {
	err := logic.Compose{}.Kill()
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonSuccessResponse(http)
	return
}
