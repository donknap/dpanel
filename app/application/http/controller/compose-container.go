package controller

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/donknap/dpanel/app/application/logic"
	logic2 "github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/notice"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/service/ws"
	"github.com/gin-gonic/gin"
	"io"
	"log/slog"
	"os"
	"path/filepath"
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
	response, err := tasker.Deploy()
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	wsBuffer := ws.NewProgressPip(fmt.Sprintf(ws.MessageTypeCompose, composeRow.ID))
	defer wsBuffer.Close()

	_, err = io.Copy(wsBuffer, response)
	if err != nil {
		slog.Error("compose", "deploy copy error", err)
	}

	// 部署完成后也上更新状态
	composeRun, err := logic.Compose{}.LsItem(tasker.Name)
	if err != nil {
		composeRow.Setting.Status = logic.ComposeStatusWaiting
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
		composeRow.Setting.Status = logic.ComposeStatusWaiting
	} else {
		composeRow.Setting.Status = composeRun.Status
	}
	_, _ = dao.Compose.Updates(composeRow)

	path := filepath.Join(filepath.Dir(tasker.Composer.Project.ComposeFiles[0]), logic.ComposeProjectDeployFileName)
	err = os.Remove(path)
	if err != nil {
		slog.Debug("compose", "delete deploy file", err, "path", path)
	}

	if function.InArray([]string{
		logic.ComposeTypeText, logic.ComposeTypeRemoteUrl,
	}, composeRow.Setting.Type) {
		dir, err := os.ReadDir(filepath.Join(storage.Local{}.GetComposePath(), composeRow.Name))
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		if len(dir) == 0 {
			err = os.RemoveAll(filepath.Join(storage.Local{}.GetComposePath(), composeRow.Name))
			if err != nil {
				slog.Debug("compose", "destroy", err)
			}
		}
	}
	if params.DeleteData {
		_, err = dao.Compose.Where(dao.Compose.ID.Eq(composeRow.ID)).Delete()
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
			slog.Debug("compose", "run log  response close", fmt.Sprintf(ws.MessageTypeComposeLog, composeRow.ID), "error", err)
			if err != nil {
				fmt.Printf("%v \n", err)
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
