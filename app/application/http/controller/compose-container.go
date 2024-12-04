package controller

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/donknap/dpanel/app/application/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/notice"
	"github.com/donknap/dpanel/common/service/ws"
	"github.com/gin-gonic/gin"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

func (self Compose) ContainerDeploy(http *gin.Context) {
	type ParamsValidate struct {
		Id                int32              `json:"id" binding:"required"`
		Environment       []accessor.EnvItem `json:"environment"`
		DeployServiceName []string           `json:"deployServiceName"`
		CreatePath        bool               `json:"createPath"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	if function.IsEmptyArray(params.DeployServiceName) {
		self.JsonResponseWithError(http, errors.New("至少选择一个服务部署"), 500)
		return
	}

	composeRow, _ := dao.Compose.Where(dao.Compose.ID.Eq(params.Id)).First()
	if composeRow == nil {
		self.JsonResponseWithError(http, errors.New("任务不存在"), 500)
		return
	}
	if !function.IsEmptyArray(params.Environment) {
		composeRow.Setting.Environment = params.Environment
	}

	tasker, err := logic.Compose{}.GetTasker(composeRow)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	// 尝试创建 compose 挂载的目录，如果运行在容器内创建也无效
	if params.CreatePath {
		for _, service := range tasker.Project().Services {
			for _, volume := range service.Volumes {
				if filepath.IsAbs(volume.Source) {
					if _, err = os.Stat(volume.Source); err != nil {
						_ = os.MkdirAll(volume.Source, os.ModePerm)
					}
				}
			}
		}
	}

	response, err := tasker.Deploy(params.DeployServiceName...)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	wsBuffer := ws.NewProgressPip(fmt.Sprintf(ws.MessageTypeCompose, composeRow.ID))
	defer wsBuffer.Close()
	wsBuffer.OnWrite = func(p string) error {
		wsBuffer.BroadcastMessage(p)
		if strings.Contains(p, "denied: You may not login") {
			_ = notice.Message{}.Error("imagePull", "拉取镜失败，仓库没有权限。")
			return errors.New("image pull denied")
		}
		return nil
	}
	_, err = io.Copy(wsBuffer, response)
	if err != nil {
		slog.Error("compose", "deploy copy error", err)
	}
	// 部署完成后也上更新状态
	composeRun, err := logic.Compose{}.LsItem(tasker.Name)
	if err != nil {
		composeRow.Setting.Status = accessor.ComposeStatusWaiting
	} else {
		composeRow.Setting.Status = composeRun.Status
	}
	_, _ = dao.Compose.Updates(composeRow)

	_ = notice.Message{}.Success("composeDeploy", composeRow.Name)
	self.JsonSuccessResponse(http)
	return
}

func (self Compose) ContainerDestroy(http *gin.Context) {
	type ParamsValidate struct {
		Id           int32 `json:"id" binding:"required"`
		DeleteImage  bool  `json:"deleteImage"`
		DeleteVolume bool  `json:"deleteVolume"`
		DeleteData   bool  `json:"deleteData"`
		DeletePath   bool  `json:"deletePath"`
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
	response, err := tasker.Destroy(params.DeleteImage, params.DeleteVolume)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	wsBuffer := ws.NewProgressPip(fmt.Sprintf(ws.MessageTypeCompose, composeRow.ID))
	defer wsBuffer.Close()

	_, err = io.Copy(wsBuffer, response)
	if err != nil {
		slog.Error("compose", "destroy copy error", err)
	}
	composeRun, err := logic.Compose{}.LsItem(tasker.Name)
	if err != nil {
		composeRow.Setting.Status = accessor.ComposeStatusWaiting
	} else {
		composeRow.Setting.Status = composeRun.Status
	}
	_, _ = dao.Compose.Updates(composeRow)

	if params.DeleteData {
		_, err = dao.Compose.Where(dao.Compose.ID.Eq(composeRow.ID)).Delete()
		if err != nil {
			slog.Debug("compose", "destroy", err)
		}
	}

	if params.DeletePath {
		if !params.DeleteData {
			self.JsonResponseWithError(http, errors.New("删除数据文件时必须同时删除任务数据"), 500)
			return
		}
		err = os.Remove(filepath.Join(filepath.Dir(composeRow.Setting.GetUriFilePath()), logic.ComposeProjectEnvFileName))
		err = os.RemoveAll(filepath.Dir(composeRow.Setting.GetUriFilePath()))
		if err != nil {
			slog.Debug("compose", "destroy", err)
		}
	}
	_ = notice.Message{}.Success("composeDestroy", composeRow.Name)
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

	response, err := tasker.Ctrl(params.Op)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	wsBuffer := ws.NewProgressPip(fmt.Sprintf(ws.MessageTypeCompose, composeRow.ID))
	defer wsBuffer.Close()

	_, err = io.Copy(wsBuffer, response)
	if err != nil {
		slog.Error("compose", "destroy copy error", err)
	}

	composeRun, err := logic.Compose{}.LsItem(tasker.Name)
	if err != nil {
		composeRow.Setting.Status = accessor.ComposeStatusWaiting
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

func (self Compose) ContainerLog(http *gin.Context) {
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
	response, err := tasker.Logs()
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	wsBuffer, err := ws.NewFdProgressPip(http, fmt.Sprintf(ws.MessageTypeComposeLog, composeRow.ID))
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	defer wsBuffer.Close()
	go func() {
		select {
		case <-wsBuffer.Done():
			err = response.Close()
			if err != nil {
				slog.Debug("compose", "run log  response close", fmt.Sprintf(ws.MessageTypeComposeLog, composeRow.ID), "error", err)
			}
		}
	}()

	wsBuffer.OnWrite = func(p string) error {
		newReader := bytes.NewReader([]byte(p))
		stdout := new(bytes.Buffer)
		_, err = stdcopy.StdCopy(stdout, stdout, newReader)
		if err != nil {
			wsBuffer.BroadcastMessage(p)
		} else {
			wsBuffer.BroadcastMessage(stdout.String())
		}
		return nil
	}
	_, err = io.Copy(wsBuffer, response)

	self.JsonSuccessResponse(http)
	return
}
