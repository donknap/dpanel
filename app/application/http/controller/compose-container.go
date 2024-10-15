package controller

import (
	"errors"
	"github.com/donknap/dpanel/app/application/logic"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/service/notice"
	"github.com/gin-gonic/gin"
)

func (self Compose) ContainerDeploy(http *gin.Context) {
	type ParamsValidate struct {
		Id int32 `json:"id" binding:"required"`
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
	err = tasker.Deploy()
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonSuccessResponse(http)
	return
}

func (self Compose) ContainerDestroy(http *gin.Context) {
	type ParamsValidate struct {
		Id          int32 `json:"id" binding:"required"`
		DeleteImage bool  `json:"deleteImage"`
		DeleteData  bool  `json:"deleteData"`
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
	err = tasker.Destroy(params.DeleteImage)
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
