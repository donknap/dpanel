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
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/exec"
	"github.com/donknap/dpanel/common/service/notice"
	"github.com/donknap/dpanel/common/service/ws"
	"github.com/gin-gonic/gin"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (self Compose) ContainerDeploy(http *gin.Context) {
	type ParamsValidate struct {
		Id                string           `json:"id" binding:"required"`
		Environment       []docker.EnvItem `json:"environment"`
		DeployServiceName []string         `json:"deployServiceName"`
		CreatePath        bool             `json:"createPath"`
		RemoveOrphans     bool             `json:"removeOrphans"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	var err error

	composeRow, _ := logic.Compose{}.Get(params.Id)
	if composeRow == nil {
		self.JsonResponseWithError(http, errors.New("任务不存在"), 500)
		return
	}
	if !function.IsEmptyArray(params.Environment) {
		composeRow.Setting.Environment = params.Environment
	}
	if !function.IsEmptyArray(params.DeployServiceName) {
		composeRow.Setting.DeployServiceName = params.DeployServiceName
	} else if !function.IsEmptyArray(composeRow.Setting.DeployServiceName) {
		params.DeployServiceName = composeRow.Setting.DeployServiceName
	}
	composeRow.Setting.UpdatedAt = time.Now().Format(function.ShowYmdHis)
	if composeRow.Setting.Status == accessor.ComposeStatusWaiting {
		composeRow.Setting.CreatedAt = time.Now().Format(function.ShowYmdHis)
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
	_ = notice.Message{}.Info(".composeDeploy", "name", composeRow.Name)

	response, err := tasker.Deploy(params.DeployServiceName, params.RemoveOrphans)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	wsBuffer := ws.NewProgressPip(fmt.Sprintf(ws.MessageTypeCompose, params.Id))
	defer wsBuffer.Close()

	wsBuffer.OnWrite = func(p string) error {
		wsBuffer.BroadcastMessage(p)
		if strings.Contains(p, "Error") {
			return errors.New(p)
		}
		return nil
	}

	_, err = io.Copy(wsBuffer, response)
	if err != nil {
		if function.ErrorHasKeyword(err, "denied: You may not login") {
			_ = notice.Message{}.Error(".imagePullInvalidAuth")
		} else if function.ErrorHasKeyword(err, "Mounts denied") {
			_ = notice.Message{}.Error(".containerMountPathDenied")
		}
		composeRow.Setting.Message = err.Error()
		composeRow.Setting.Status = accessor.ComposeStatusError
	} else {
		composeRow.Setting.Message = ""
		composeRow.Setting.Status = ""
	}
	if composeRow.ID > 0 {
		_, _ = dao.Compose.Updates(composeRow)
	}
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	self.JsonSuccessResponse(http)
	return
}

func (self Compose) ContainerDestroy(http *gin.Context) {
	type ParamsValidate struct {
		Id           string `json:"id" binding:"required"`
		DeleteImage  bool   `json:"deleteImage"`
		DeleteVolume bool   `json:"deleteVolume"`
		DeleteData   bool   `json:"deleteData"`
		DeletePath   bool   `json:"deletePath"`
	}

	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	composeRow, _ := logic.Compose{}.Get(params.Id)
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

	wsBuffer := ws.NewProgressPip(fmt.Sprintf(ws.MessageTypeCompose, params.Id))
	defer wsBuffer.Close()

	_, err = io.Copy(wsBuffer, response)
	if err != nil {
		slog.Error("compose", "destroy copy error", err)
	}

	if params.DeleteData {
		_, err = dao.Compose.Where(dao.Compose.ID.Eq(composeRow.ID)).Delete()
		if err != nil {
			slog.Debug("compose", "destroy", err)
		}
	} else {
		composeRow.Setting.DeployServiceName = make([]string, 0)
		composeRow.Setting.Status = ""
		composeRow.Setting.CreatedAt = ""
		composeRow.Setting.UpdatedAt = ""
		_, _ = dao.Compose.Updates(composeRow)
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
	_ = notice.Message{}.Info(".composeDestroy", "name", composeRow.Name)

	self.JsonSuccessResponse(http)
	return
}

func (self Compose) ContainerCtrl(http *gin.Context) {
	type ParamsValidate struct {
		Id string `json:"id" binding:"required"`
		Op string `json:"op" binding:"required" oneof:"start restart stop pause unpause ls"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	composeRow, _ := logic.Compose{}.Get(params.Id)
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
	wsBuffer := ws.NewProgressPip(fmt.Sprintf(ws.MessageTypeCompose, params.Id))
	defer wsBuffer.Close()

	_, err = io.Copy(wsBuffer, response)
	if err != nil {
		slog.Error("compose", "destroy copy error", err)
	}

	self.JsonSuccessResponse(http)
	return
}

func (self Compose) ContainerProcessKill(http *gin.Context) {
	err := exec.Kill()
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonSuccessResponse(http)
	return
}

func (self Compose) ContainerLog(http *gin.Context) {
	type ParamsValidate struct {
		Id        string `json:"id" binding:"required"`
		LineTotal int    `json:"lineTotal" binding:"required,number,oneof=50 100 200 500 1000 5000 -1"`
		Download  bool   `json:"download"`
		ShowTime  bool   `json:"showTime"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	composeRow, _ := logic.Compose{}.Get(params.Id)
	if composeRow == nil {
		self.JsonResponseWithError(http, notice.Message{}.New(".commonDataNotFoundOrDeleted"), 500)
		return
	}
	tasker, err := logic.Compose{}.GetTasker(composeRow)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	follow := true
	if params.Download {
		follow = false
	}
	wsBuffer := ws.NewProgressPip(fmt.Sprintf(ws.MessageTypeComposeLog, params.Id))

	response, err := tasker.Logs(params.LineTotal, params.ShowTime, follow)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	if params.Download {
		buffer, err := io.ReadAll(response)
		_ = response.Close()
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		http.Header("Content-Type", "text/plain")
		http.Header("Content-Disposition", "attachment; filename="+params.Id+".log")
		http.Data(200, "text/plain", buffer)
		return
	}
	go func() {
		select {
		case <-wsBuffer.Done():
			err = response.Close()
			if err != nil {
				slog.Debug("compose", "run log  response close", fmt.Sprintf(ws.MessageTypeComposeLog, params.Id), "error", err)
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
