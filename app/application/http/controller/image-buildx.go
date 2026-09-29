package controller

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/donknap/dpanel/app/application/logic/task"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/docker/buildx"
	buildxcontext "github.com/donknap/dpanel/common/service/docker/buildx/context"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/service/ws"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
)

type ImageBuildx struct {
	controller.Abstract
}

func (self ImageBuildx) Build(http *gin.Context) {
	type ParamsValidate struct {
		Id int32 `json:"id" binding:"required,gt=0"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	imageRow, err := dao.Image.Where(dao.Image.ID.Eq(params.Id)).First()
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	if imageRow == nil || imageRow.Setting == nil {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted), 500)
		return
	}
	sdk, err := docker.NewClientWithUser(http)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	progress, owner, err := ws.NewFdProgressPip(http, sdk.Name, fmt.Sprintf(ws.MessageTypeImageBuild, params.Id))
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	if !owner {
		self.JsonResponseWithError(http, errors.New("image build is already running"), 409)
		return
	}
	defer progress.Close()
	stopWatchRequest := context.AfterFunc(http.Request.Context(), progress.Close)
	defer stopWatchRequest()
	if err := progress.Context().Err(); err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	if function.IsEmptyArray(imageRow.Setting.Tags) {
		tag := imageRow.Setting.Tag
		if tag == "" {
			tag = imageRow.Tag
		}
		if tag != "" {
			imageRow.Setting.Tags = []accessor.ImageSettingTag{{
				Tag:    function.ImageTag(tag),
				Enable: true,
			}}
		}
	}
	if imageRow.Setting.BuildDockerfileContent == "" {
		imageRow.Setting.BuildDockerfileContent = imageRow.Setting.BuildDockerfile
	}
	if imageRow.Setting.BuildDockerfileRoot == "" {
		imageRow.Setting.BuildDockerfileRoot = imageRow.Setting.BuildRoot
	}
	imageRow.Status = define.DockerImageBuildStatusProcess
	imageRow.Message = ""
	if err := dao.Image.Save(imageRow); err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	buildSetting := *imageRow.Setting
	if buildSetting.BuildZip != "" && !filepath.IsAbs(buildSetting.BuildZip) {
		buildSetting.BuildZip = storage.Local{}.GetSaveRealPath(buildSetting.BuildZip)
	}
	startTime := time.Now()
	log, imageID, err := func() (string, string, error) {
		state, err := buildxcontext.Get(sdk)
		if err != nil {
			return "", "", err
		}
		if !state.Exists {
			if err := buildxcontext.Create(sdk, buildxcontext.CreateOption{}); err != nil {
				return "", "", err
			}
		}
		if err := progress.Context().Err(); err != nil {
			return "", "", err
		}
		options, err := (task.Docker{}).BuildxOptions(buildSetting)
		if err != nil {
			return "", "", err
		}
		builder, err := buildx.New(progress.Context(), sdk, options...)
		if err != nil {
			return "", "", err
		}
		defer builder.Close()
		return builder.Run(progress)
	}()
	if ctxErr := progress.Context().Err(); ctxErr != nil {
		err = ctxErr
		imageRow.Status = define.DockerImageBuildStatusStop
	} else if err != nil {
		imageRow.Status = define.DockerImageBuildStatusError
	} else {
		imageRow.Status = define.DockerImageBuildStatusSuccess
	}
	imageRow.Setting.ImageId = imageID
	imageRow.Setting.UseTime = time.Since(startTime).Seconds()
	imageRow.Message = log
	if saveErr := dao.Image.Save(imageRow); saveErr != nil {
		err = errors.Join(err, saveErr)
	}
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonResponseWithoutError(http, gin.H{"id": imageRow.ID})
}

func (self ImageBuildx) GetContext(http *gin.Context) {
	sdk, err := docker.NewClientWithUser(http)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	state, err := buildxcontext.Get(sdk)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonResponseWithoutError(http, gin.H{
		"name":   state.Name,
		"detail": state.Detail,
		"config": state.Config,
		"proxy":  state.Proxy,
		"exists": state.Exists,
	})
}

func (self ImageBuildx) CreateContext(http *gin.Context) {
	type ParamsValidate struct {
		Config *string `json:"config"`
		Proxy  string  `json:"proxy"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	sdk, err := docker.NewClientWithUser(http)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	if err := buildxcontext.Create(sdk, buildxcontext.CreateOption{Config: params.Config, Proxy: params.Proxy}); err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonSuccessResponse(http)
}

func (self ImageBuildx) CleanContext(http *gin.Context) {
	type ParamsValidate struct {
		EnablePrune       bool  `json:"enablePrune"`
		EnableRemove      bool  `json:"enableRemove"`
		EnableForceRemove *bool `json:"enableForceRemove"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	sdk, err := docker.NewClientWithUser(http)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	if params.EnablePrune {
		if err := buildxcontext.Prune(sdk); err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}
	if params.EnableRemove {
		force := params.EnableForceRemove == nil || *params.EnableForceRemove
		if err := buildxcontext.Remove(sdk, force); err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}
	self.JsonSuccessResponse(http)
}
