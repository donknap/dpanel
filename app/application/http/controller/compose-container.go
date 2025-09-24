package controller

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/docker/docker/api/types/container"
	"github.com/donknap/dpanel/app/application/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/notice"
	"github.com/donknap/dpanel/common/service/ws"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/donknap/dpanel/common/types/event"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

func (self Compose) ContainerDeploy(http *gin.Context) {
	type ParamsValidate struct {
		Id                string           `json:"id" binding:"required"`
		Environment       []docker.EnvItem `json:"environment"`
		DeployServiceName []string         `json:"deployServiceName"`
		CreatePath        bool             `json:"createPath"`
		RemoveOrphans     bool             `json:"removeOrphans"`
		PullImage         bool             `json:"pullImage"`
		Build             bool             `json:"build"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	var err error

	composeRow, _ := logic.Compose{}.Get(params.Id)
	if composeRow == nil {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted), 500)
		return
	}

	if !function.IsEmptyArray(params.DeployServiceName) {
		composeRow.Setting.DeployServiceName = params.DeployServiceName
	}
	tasker, warning, err := logic.Compose{}.GetTasker(&entity.Compose{
		Name: composeRow.Name,
		Setting: &accessor.ComposeSettingOption{
			Type:          composeRow.Setting.Type,
			Uri:           composeRow.Setting.Uri,
			RemoteUrl:     composeRow.Setting.RemoteUrl,
			Environment:   params.Environment,
			DockerEnvName: composeRow.Setting.DockerEnvName,
			RunName:       composeRow.Setting.RunName,
		},
	})
	if err != nil || warning != nil {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageComposeParseYamlIncorrect, "error", errors.Join(warning, err).Error()), 500)
		return
	}

	// 添加禁用服务，只有部署的时候需要，避免在获取详情时拿不到全部服务
	if !function.IsEmptyArray(composeRow.Setting.DeployServiceName) {
		services, err := tasker.Project.GetServices()
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		for _, item := range services {
			if !function.InArray(composeRow.Setting.DeployServiceName, item.Name) {
				tasker.Project = tasker.Project.WithServicesDisabled(item.Name)
			}
		}
	}

	// 尝试创建 compose 挂载的目录，如果运行在容器内创建也无效
	if params.CreatePath {
		for _, service := range tasker.Project.Services {
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

	progress := ws.NewProgressPip(fmt.Sprintf(ws.MessageTypeCompose, params.Id))
	defer progress.Close()

	var response io.ReadCloser
	if params.Build {
		response, err = tasker.Build()
	} else {
		response, err = tasker.Deploy(params.RemoveOrphans, params.PullImage)
	}
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	go func() {
		<-progress.Done()
		_ = response.Close()
	}()

	progress.OnWrite = func(p string) error {
		progress.BroadcastMessage(p)
		return nil
	}

	_, err = io.Copy(progress, response)
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
		_ = dao.Compose.Save(composeRow)
	}

	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	// 查看当前任务下的容器 hash 值是否部署成功，并写入 dpanel 的标识用于查找
	runCompose := logic.Compose{}.LsItem(composeRow.Name)
	if runCompose == nil || len(runCompose.ContainerList) != len(tasker.Project.Services) {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageComposeDeployIncorrect), 500)
		return
	}

	for _, item := range runCompose.ContainerList {
		if item.Container.State != container.StateRunning {
			self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageComposeDeployIncorrect), 500)
			return
		}
	}

	// 这里需要单独适配一下 php 环境的相关扩展安装
	// 目前只有 php 需要这样处理，暂时先直接进行判断
	if strings.HasPrefix(composeRow.Setting.Store, accessor.StoreTypeOnePanel) && strings.HasSuffix(composeRow.Setting.Store, "@php") {
		if phpExt, _, ok := function.PluckArrayItemWalk(params.Environment, func(item docker.EnvItem) bool {
			return item.Name == "PHP_EXTENSIONS"
		}); ok {
			out, err := docker.Sdk.ContainerExec(progress.Context(), runCompose.ContainerList[0].Container.ID, container.ExecOptions{
				Privileged:   true,
				Tty:          false,
				AttachStdin:  false,
				AttachStdout: true,
				AttachStderr: false,
				Cmd: []string{
					"install-ext",
					phpExt.Value,
				},
			})
			if err != nil {
				self.JsonResponseWithError(http, err, 500)
				return
			}
			defer func() {
				out.Close()
			}()
			_, err = io.Copy(progress, out.Reader)
			if err != nil {
				self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageComposeDeployIncorrect), 500)
				return
			}
		}
	}

	self.JsonSuccessResponse(http)
	return
}

func (self Compose) ContainerDestroy(http *gin.Context) {
	type ParamsValidate struct {
		Id                 string   `json:"id" binding:"required"`
		DeleteImage        bool     `json:"deleteImage"`
		DeleteVolume       bool     `json:"deleteVolume"`
		DeleteData         bool     `json:"deleteData"`
		DeletePath         bool     `json:"deletePath"`
		DestroyServiceName []string `json:"destroyServiceName"`
	}

	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	composeRow, _ := logic.Compose{}.Get(params.Id)
	if composeRow == nil {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted), 500)
		return
	}
	runCompose := logic.Compose{}.LsItem(composeRow.Name)
	if runCompose != nil && len(runCompose.ContainerList) != 0 {
		tasker, _, err := logic.Compose{}.GetTasker(composeRow)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}

		if !function.IsEmptyArray(params.DestroyServiceName) {
			for _, item := range params.DestroyServiceName {
				tasker.Project = tasker.Project.WithServicesDisabled(item)
			}
		}

		progress := ws.NewProgressPip(fmt.Sprintf(ws.MessageTypeCompose, params.Id))
		defer progress.Close()

		response, err := tasker.Destroy(params.DeleteImage, params.DeleteVolume)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		go func() {
			<-progress.Done()
			_ = response.Close()
		}()
		_, err = io.Copy(progress, response)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}

	if params.DeleteData {
		_, err := dao.Compose.Where(dao.Compose.ID.Eq(composeRow.ID)).Delete()
		if err != nil {
			slog.Debug("compose", "destroy", err)
		} else {
			facade.GetEvent().Publish(event.ComposeDeleteEvent, event.ComposePayload{
				Compose: composeRow,
				Ctx:     http,
			})
		}
	} else {
		composeRow.Setting.DeployServiceName = make([]string, 0)
		composeRow.Setting.Status = ""
		_ = dao.Compose.Save(composeRow)
	}

	if params.DeletePath {
		if !params.DeleteData {
			self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageComposeDeleteFileMustDeleteTask), 500)
			return
		}
		err := os.RemoveAll(filepath.Dir(composeRow.Setting.GetUriFilePath()))
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
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted), 500)
		return
	}
	tasker, _, err := logic.Compose{}.GetTasker(composeRow)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	progress := ws.NewProgressPip(fmt.Sprintf(ws.MessageTypeCompose, params.Id))
	defer progress.Close()

	response, err := tasker.Ctrl(params.Op)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	go func() {
		<-progress.Done()
		_ = response.Close()
	}()
	_, err = io.Copy(progress, response)
	if err != nil {
		slog.Error("compose", "destroy copy error", err)
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
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted), 500)
		return
	}
	tasker, _, err := logic.Compose{}.GetTasker(composeRow)
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
		stdout, err := function.CombinedStdout(newReader)
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
