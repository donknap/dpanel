package controller

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/donknap/dpanel/app/application/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/docker/types"
	"github.com/donknap/dpanel/common/service/exec"
	containerexec "github.com/donknap/dpanel/common/service/exec/container"
	"github.com/donknap/dpanel/common/service/notice"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/service/ws"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/donknap/dpanel/common/types/event"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
)

func (self Compose) ContainerDeploy(http *gin.Context) {
	type ParamsValidate struct {
		Id                string          `json:"id" binding:"required"`
		BackupId          string          `json:"backupId"`
		Environment       []types.EnvItem `json:"environment"`
		DeployServiceName []string        `json:"deployServiceName"`
		CreatePath        bool            `json:"createPath"`
		RemoveOrphans     bool            `json:"removeOrphans"`
		PullImage         bool            `json:"pullImage"`
		Build             bool            `json:"build"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	var err error
	dockerSdk, err := docker.NewClientWithUser(http)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	composeRow, err := logic.Compose{}.Get(params.Id)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	if composeRow.Setting.DockerEnvName != dockerSdk.DockerEnv.GetComposeEnvName() {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted), 500)
		return
	}
	backup, err := logic.GetComposeBackup(params.BackupId, composeRow)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	defer func() {
		if err != nil {
			if backup != nil {
				if restoreErr := backup.Restore(); restoreErr != nil {
					err = errors.Join(err, restoreErr)
				}
			}
			self.JsonResponseWithError(http, err, 500)
			return
		}
		if backup != nil {
			storage.Cache.Delete(fmt.Sprintf(storage.CacheKeyComposeBackup, backup.Id))
		}
		self.JsonSuccessResponse(http)
	}()

	if !function.IsEmptyArray(params.DeployServiceName) {
		composeRow.Setting.DeployServiceName = params.DeployServiceName
	}
	tasker, _, err := logic.Compose{}.GetTasker(dockerSdk, &entity.Compose{
		Name: composeRow.Name,
		Setting: &accessor.ComposeSettingOption{
			Type:          composeRow.Setting.Type,
			Delegate:      composeRow.Setting.Delegate,
			Uri:           composeRow.Setting.Uri,
			RemoteUrl:     composeRow.Setting.RemoteUrl,
			Environment:   params.Environment,
			UpdatedAt:     composeRow.Setting.UpdatedAt,
			DockerEnvName: composeRow.Setting.DockerEnvName,
			RunName:       composeRow.Setting.RunName,
		},
	})
	if err != nil {
		return
	}
	if err = (logic.Compose{}).ValidateRelativeBindMounts(tasker); err != nil {
		return
	}

	// 添加禁用服务，只有部署的时候需要，避免在获取详情时拿不到全部服务
	if !function.IsEmptyArray(composeRow.Setting.DeployServiceName) {
		serviceList, serviceErr := tasker.Project.GetServices()
		if serviceErr != nil {
			err = serviceErr
			return
		}
		for _, item := range serviceList {
			if !function.InArray(composeRow.Setting.DeployServiceName, item.Name) {
				tasker.Project = tasker.Project.WithServicesDisabled(item.Name)
			}
		}
	}

	if params.CreatePath {
		tasker.CreateBindPath()
	}

	var response io.ReadCloser
	if params.Build {
		response, err = tasker.Build()
	} else {
		response, err = tasker.Deploy(params.RemoveOrphans, params.PullImage)
	}
	if err != nil {
		return
	}

	progress := ws.NewProgressPip(fmt.Sprintf(ws.MessageTypeCompose, params.Id))
	defer progress.Close()
	go func() {
		<-progress.Done()
		_ = response.Close()
	}()

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
		err = function.ErrorMessage(define.ErrorMessageCommonDeployIncorrect)
		return
	}

	// 查看当前任务下的容器 hash 值是否部署成功
	runCompose := logic.Compose{}.LsItem(composeRow.Name)
	if runCompose == nil || len(runCompose.ContainerList) != len(tasker.Project.Services) {
		err = function.ErrorMessage(define.ErrorMessageCommonDeployIncorrect)
		return
	}

	for _, item := range runCompose.ContainerList {
		if item.Container.State != container.StateRunning {
			err = function.ErrorMessage(define.ErrorMessageCommonDeployIncorrect)
			return
		}
	}

	// 如果当前容器配置过转发，则加入 dpanel-local 网络
	for _, item := range runCompose.ContainerList {
		if row, err := dao.SiteDomain.Where(dao.SiteDomain.ContainerID.In(item.Container.Names...)).First(); err == nil {
			_ = dockerSdk.NetworkConnect(dockerSdk.Ctx, types.NetworkItem{
				Name: define.DPanelProxyNetworkName,
			}, row.ContainerID)
		}
	}

	// 这里需要单独适配一下 php 环境的相关扩展安装
	// 目前只有 php 需要这样处理，暂时先直接进行判断
	if strings.HasPrefix(composeRow.Setting.Store, define.StoreTypeOnePanel) && strings.HasSuffix(composeRow.Setting.Store, "@php") {
		_ = dockerSdk.NetworkConnect(dockerSdk.Ctx, types.NetworkItem{
			Name: define.DPanelProxyNetworkName,
		}, runCompose.ContainerList[0].Container.Names[0])

		_, _ = progress.Write([]byte("Start install PHP_EXTENSIONS \n"))
		if phpExt, _, ok := function.PluckArrayItemWalk(params.Environment, func(item types.EnvItem) bool {
			return item.Name == "PHP_EXTENSIONS"
		}); ok {
			_, _ = progress.Write([]byte("Install PHP_EXTENSIONS " + phpExt.String() + "\n"))
			var cmd exec.Executor
			cmd, err = containerexec.New(
				containerexec.WithDockerClient(dockerSdk.Client),
				containerexec.WithContainerName(runCompose.ContainerList[0].Container.ID),
				containerexec.WithCommandName("install-ext"),
				containerexec.WithArgs(phpExt.Value),
				containerexec.WithCtx(progress.Context()),
				containerexec.WithExecOptions(container.ExecOptions{Privileged: true}),
			)
			if err != nil {
				return
			}
			defer cmd.Close()
			var out io.ReadCloser
			out, err = cmd.RunInReadPip()
			if err != nil {
				return
			}
			defer out.Close()
			_, err = io.Copy(progress, out)
			if err != nil {
				err = function.ErrorMessage(define.ErrorMessageCommonDeployIncorrect)
				return
			}
		}
	}

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
	dockerSdk, err := docker.NewClientWithUser(http)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	dockerEnvName := dockerSdk.DockerEnv.GetComposeEnvName()
	composeRow, err := logic.Compose{}.Get(params.Id)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	runCompose := logic.Compose{}.LsItem(composeRow.Name)
	if composeRow.Setting.DockerEnvName != dockerEnvName {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted), 500)
		return
	}
	composePath := storage.Local{}.GetComposePath(composeRow.Setting.DockerEnvName)
	var targetPath string
	if params.DeletePath {
		if composeRow.ID > 0 && !params.DeleteData {
			self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageComposeDeleteFileMustDeleteTask), 500)
			return
		}
		targetPath = composeRow.Name
		if composeRow.Setting.Type != accessor.ComposeTypeOutPath {
			composeFile, _, err := composeRow.Setting.GetUriPath(composeRow.Name)
			if err != nil {
				self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted), 500)
				return
			}
			targetPath, err = filepath.Rel(composePath, filepath.Dir(composeFile))
			if err != nil {
				self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted), 500)
				return
			}
		}
		if targetPath == "." || !filepath.IsLocal(targetPath) {
			self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted), 500)
			return
		}
	}
	if runCompose != nil && len(runCompose.ContainerList) != 0 {
		progress := ws.NewProgressPip(fmt.Sprintf(ws.MessageTypeCompose, params.Id))
		defer progress.Close()
		ctx, cancel := context.WithCancel(dockerSdk.Ctx)
		defer cancel()
		stop := context.AfterFunc(progress.Context(), cancel)
		defer stop()
		if err = dockerSdk.ComposeRemove(ctx, runCompose.RunName, docker.ComposeRemoveOption{
			CleanNetwork:    len(params.DestroyServiceName) == 0,
			CleanVolume:     params.DeleteVolume,
			CleanImage:      params.DeleteImage,
			ServiceNameList: params.DestroyServiceName,
			Output:          progress,
		}); err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}

	if params.DeleteData && composeRow.ID > 0 {
		_, err := dao.Compose.Where(dao.Compose.ID.Eq(composeRow.ID)).Delete()
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		facade.GetEvent().Publish(event.ComposeDeleteEvent, event.ComposePayload{
			Compose: composeRow,
			Ctx:     http,
		})
	} else if composeRow.ID > 0 {
		composeRow.Setting.DeployServiceName = make([]string, 0)
		composeRow.Setting.Status = ""
		if err = dao.Compose.Save(composeRow); err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}

	if params.DeletePath {
		sourceDir := filepath.Join(composePath, targetPath)
		if composeRow.ID > 0 {
			if function.IsRunInDocker() {
				var targetDir string
				if composeRow.Setting.Type == accessor.ComposeTypeOutPath {
					if composeRow.Setting.IsDelegate() && len(composeRow.Setting.Uri) > 0 {
						targetDir = path.Dir(function.PathSeparatorToSlash(composeRow.Setting.Uri[0]))
					}
				} else if _, hostDir := (logic.Compose{}).ComposeDir(composeRow.Setting.DockerEnvName); hostDir != "" {
					targetDir = path.Join(hostDir, filepath.ToSlash(targetPath))
				}
				if targetDir != "" {
					workingDir := function.SlashPathToSystem(targetDir)
					if hostDir, ok := function.WindowsPathToSlash(targetDir); ok {
						workingDir = filepath.Join("/mnt/host", strings.TrimPrefix(hostDir, "/"))
					}
					if err = (logic.Compose{}).DeleteLocalLink(sourceDir, workingDir); err != nil {
						self.JsonResponseWithError(http, err, 500)
						return
					}
				}
			}
			if dockerSdk.DockerEnv.IsRemote() && composeRow.Setting.Type != accessor.ComposeTypeOutPath {
				if err = (logic.Compose{}).DeleteRemoteDir(http, dockerSdk, sourceDir); err != nil {
					self.JsonResponseWithError(http, err, 500)
					return
				}
			}
		}
		composeRoot, err := os.OpenRoot(composePath)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		if err == nil {
			defer composeRoot.Close()
			if err = composeRoot.RemoveAll(targetPath); err != nil {
				self.JsonResponseWithError(http, err, 500)
				return
			}
		}
	}

	if composeRow.ID > 0 || runCompose != nil {
		_ = notice.Message{}.Info(".composeDestroy", "name", composeRow.Name)
	}

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
	dockerSdk, err := docker.NewClientWithUser(http)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	composeRow, err := logic.Compose{}.Get(params.Id)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	tasker, _, err := logic.Compose{}.GetTasker(dockerSdk, composeRow)
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
	dockerSdk, err := docker.NewClientWithUser(http)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	composeRow, err := logic.Compose{}.Get(params.Id)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	tasker, _, err := logic.Compose{}.GetTasker(dockerSdk, composeRow)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	if params.Download {
		response, err := tasker.Logs(params.LineTotal, params.ShowTime, false)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
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
	progress, owner, err := ws.NewFdProgressPip(http, docker.Sdk.Name, fmt.Sprintf(ws.MessageTypeComposeLog, params.Id))
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	response, err := tasker.Logs(params.LineTotal, params.ShowTime, owner)
	if err != nil {
		if owner {
			progress.Close()
		}
		self.JsonResponseWithError(http, err, 500)
		return
	}
	defer func() {
		_ = response.Close()
	}()
	go func() {
		select {
		case <-http.Request.Context().Done():
		case <-progress.Done():
		}
		if err := response.Close(); err != nil {
			slog.Warn("compose run log progress close", "id", fmt.Sprintf(ws.MessageTypeComposeLog, params.Id), "error", err)
		}
	}()

	progress.OnWrite = func(p string) error {
		progress.BroadcastMessage(p)
		return nil
	}
	_, err = io.Copy(progress, response)

	self.JsonSuccessResponse(http)
	return
}
