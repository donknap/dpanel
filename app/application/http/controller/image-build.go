package controller

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/docker/docker/api/types/build"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/go-units"
	"github.com/donknap/dpanel/app/application/logic"
	"github.com/donknap/dpanel/app/application/logic/task"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/docker/types"
	"github.com/donknap/dpanel/common/service/exec/local"
	"github.com/donknap/dpanel/common/service/notice"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/service/ws"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
)

type ImageBuild struct {
	controller.Abstract
}

func (self ImageBuild) Create(http *gin.Context) {
	type ParamsValidate struct {
		Id       int32  `json:"id"`
		Title    string `json:"title"`
		OnlySave bool   `json:"onlySave"`
		accessor.ImageSettingOption
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	if params.BuildDockerfileContent == "" && params.BuildZip == "" && params.BuildGit == "" && params.BuildPath == "" {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageImageBuildTypeEmpty), 500)
		return
	}
	if params.BuildZip != "" && params.BuildGit != "" {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageImageBuildTypeConflict), 500)
		return
	}

	if params.BuildZip != "" {
		path := storage.Local{}.GetSaveRealPath(params.BuildZip)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonUploadFileEmpty), 500)
			return
		}
		params.BuildZip = path
	}

	if params.BuildPath != "" {
		if _, err := os.Stat(params.BuildPath); err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}

	params.Tags = function.PluckArrayWalk(params.Tags, func(item accessor.ImageSettingTag) (accessor.ImageSettingTag, bool) {
		item.Tag = function.ImageTag(fmt.Sprintf("%s/%s", item.Registry, item.Name))
		return item, true
	})

	params.BuildSecret = function.PluckArrayWalk(params.BuildSecret, func(item types.EnvItem) (types.EnvItem, bool) {
		if v, err := function.RSAEncode(item.Value); err == nil {
			item.Value = v
		}
		return item, true
	})

	imageNew := &entity.Image{
		Tag:       "",
		BuildType: "",
		Title:     params.Title,
		Setting:   &params.ImageSettingOption,
		Status:    define.DockerImageBuildStatusStop,
		Message:   "",
	}
	if imageRow, _ := dao.Image.Where(dao.Image.ID.Eq(params.Id)).First(); imageRow != nil {
		imageNew.ID = imageRow.ID
		imageNew.Status = imageRow.Status
		imageNew.Message = imageRow.Message
	}
	_ = dao.Image.Save(imageNew)

	if !params.OnlySave {
		var log string
		var err error
		var imageId string

		startTime := time.Now()
		messageId := fmt.Sprintf(ws.MessageTypeImageBuild, params.Id)
		if params.BuildEngine == define.ImageBuildBuildX {
			log, err = task.Docker{}.ImageBuildX(messageId, params.ImageSettingOption)
			if err != nil {
				self.JsonResponseWithError(http, err, 500)
				return
			}
			// 检测是否成功
			matches := regexp.MustCompile(`"containerimage\.digest"\s*:\s*"(sha256:[a-f0-9]+)"`).FindAllStringSubmatch(log, -1)
			imageId = strings.Join(function.PluckArrayWalk(matches, func(item []string) (string, bool) {
				return item[1], true
			}), "-")
			if imageId == "" {
				self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageImageBuildError, "message", ""), 500)
				return
			}
		} else {
			log, err = task.Docker{}.ImageBuild(docker.Sdk, messageId, params.ImageSettingOption)
			if params.ImageSettingOption.BuildEnablePush {
				wsBuffer := ws.NewProgressPip(messageId)
				defer wsBuffer.Close()
				for _, tag := range params.ImageSettingOption.Tags {
					pushOption := image.PushOptions{}
					if v := (logic.Image{}).GetRegistryConfig(tag.Registry); v != nil {
						pushOption.RegistryAuth = v.AuthString()
					}
					reader, err := docker.Sdk.Client.ImagePush(docker.Sdk.Ctx, tag.Uri(), pushOption)
					if err != nil {
						self.JsonResponseWithError(http, err, 500)
						return
					}
					_, err = io.Copy(wsBuffer, reader)
					if err != nil {
						self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonCancelOperator, "message", err.Error()), 500)
						return
					}
				}
			}
			matches := regexp.MustCompile(`Successfully built\s*([a-f0-9]+)`).FindAllStringSubmatch(log, -1)
			imageId = strings.Join(function.PluckArrayWalk(matches, func(item []string) (string, bool) {
				return item[1], true
			}), "-")
		}
		if err != nil {
			imageNew.Status = define.DockerImageBuildStatusError
		} else {
			imageNew.Status = define.DockerImageBuildStatusSuccess
		}

		imageNew.Setting.ImageId = imageId
		imageNew.Setting.UseTime = time.Now().Sub(startTime).Seconds()
		imageNew.Message = log
		_ = dao.Image.Save(imageNew)

		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}

	self.JsonResponseWithoutError(http, gin.H{
		"id": imageNew.ID,
	})
	return
}

func (self ImageBuild) GetDetail(http *gin.Context) {
	type ParamsValidate struct {
		Id int32 `json:"id" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	imageRow, _ := dao.Image.Where(dao.Image.ID.Eq(params.Id)).First()
	if imageRow == nil {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted), 500)
		return
	}
	if function.IsEmptyArray(imageRow.Setting.Tags) {
		tag := imageRow.Setting.Tag
		if tag == "" {
			tag = imageRow.Tag
		}
		tagDetail := function.ImageTag(tag)
		imageRow.Setting.Tag = tagDetail.Name
		imageRow.Setting.Tags = []accessor.ImageSettingTag{
			{
				Enable: true,
				Tag:    tagDetail,
			},
		}
	}

	imageRow.Setting.BuildSecret = function.PluckArrayWalk(imageRow.Setting.BuildSecret, func(item types.EnvItem) (types.EnvItem, bool) {
		if v, err := function.RSADecode(item.Value, nil); err == nil {
			item.Value = v
		}
		return item, true
	})

	if imageRow.Setting.BuildType == "" {
		imageRow.Setting.BuildType = imageRow.BuildType
	}
	if imageRow.Setting.BuildDockerfileContent == "" {
		imageRow.Setting.BuildDockerfileContent = imageRow.Setting.BuildDockerfile
	}
	if imageRow.Setting.BuildDockerfileRoot == "" {
		imageRow.Setting.BuildDockerfileRoot = imageRow.Setting.BuildRoot
	}

	self.JsonResponseWithoutError(http, gin.H{
		"detail": imageRow,
	})
	return
}

func (self ImageBuild) Delete(http *gin.Context) {
	type ParamsValidate struct {
		Id []int32 `json:"id" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	_, err := dao.Image.Where(dao.Image.ID.In(params.Id...)).Delete()
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	self.JsonSuccessResponse(http)
	return
}

func (self ImageBuild) GetList(http *gin.Context) {
	list, err := dao.Image.Order(dao.Image.ID.Desc()).Where(dao.Image.Setting.IsNotNull()).Find()
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	list = function.PluckArrayWalk(list, func(item *entity.Image) (*entity.Image, bool) {
		if function.IsEmptyArray(item.Setting.Tags) {

			item.Setting.Tags = []accessor.ImageSettingTag{
				{
					Tag:    function.ImageTag(item.Setting.Tag),
					Enable: true,
				},
			}
		}
		return item, true
	})
	self.JsonResponseWithoutError(http, gin.H{
		"list": list,
	})
	return
}

func (self ImageBuild) Prune(http *gin.Context) {
	res, err := docker.Sdk.Client.BuildCachePrune(docker.Sdk.Ctx, build.CachePruneOptions{
		All: true,
	})
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	_ = notice.Message{}.Info(".imageBuildPrune", "size", units.HumanSize(float64(res.SpaceReclaimed)))
	self.JsonSuccessResponse(http)
	return
}

func (self ImageBuild) Buildx(http *gin.Context) {
	type ParamsValidate struct {
		Config       *string `json:"config"`
		EnableCreate bool    `json:"enableCreate"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	imageLogic := logic.Image{}
	createConfigOption, err := imageLogic.BuildxConfig(docker.Sdk.Name)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	builderName := fmt.Sprintf(define.DockerBuilderName, docker.Sdk.Name)
	contextName := fmt.Sprintf(define.DockerContextName, docker.Sdk.Name)
	description := fmt.Sprintf("Created by DPanel DO NOT DELETE!!! %s", function.Sha256Struct(docker.Sdk.DockerEnv))

	recreateContext := true
	if result, err := local.QuickRun("docker context inspect", contextName); err == nil && strings.Contains(string(result), description) {
		recreateContext = false
	}
	if params.EnableCreate || recreateContext {
		if params.Config != nil {
			createConfigOption.ConfigContent = params.Config
			if err := imageLogic.BuildxCreateConfig(createConfigOption); err != nil {
				self.JsonResponseWithError(http, err, 500)
				return
			}
		} else if _, err := os.Stat(createConfigOption.ConfigPath); os.IsNotExist(err) {
			createConfigOption.DriverHttpProxy = os.Getenv("HTTP_PROXY")
			if err := imageLogic.BuildxCreateConfig(createConfigOption); err != nil {
				self.JsonResponseWithError(http, err, 500)
				return
			}
		} else if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}

		if recreateContext {
			if _, err := docker.Sdk.RunResult("context", "rm", contextName, "--force"); err != nil {
				self.JsonResponseWithError(http, err, 500)
				return
			}
			cmd, err := docker.Sdk.Run("context", "create", contextName, "--description", description)
			if err != nil {
				self.JsonResponseWithError(http, err, 500)
				return
			}
			if _, err = cmd.RunWithResult(); err != nil {
				self.JsonResponseWithError(http, err, 500)
				return
			}
		}
		if _, err := docker.Sdk.RunResult("buildx", "rm", contextName+"-builder", "--force"); err != nil {
			slog.Debug("image build rm buildx", "error", err)
		}
		createArgs := []string{
			"buildx", "create",
			"--name", builderName,
			"--driver", "docker-container",
			"--driver-opt", "network=host",
			"--buildkitd-config", createConfigOption.ConfigPath,
		}
		createArgs = append(createArgs, "--bootstrap", contextName)
		if _, err = docker.Sdk.RunResult(createArgs...); err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}

	var detail string
	if v, err := docker.Sdk.RunResult("buildx", "inspect", fmt.Sprintf(define.DockerBuilderName, docker.Sdk.Name)); err == nil {
		detail = string(v)
	} else {
		slog.Info("buildx get inspect", "error", err)
	}
	var config string
	if v, err := os.ReadFile(createConfigOption.ConfigPath); err == nil {
		config = string(v)
	} else if !os.IsNotExist(err) {
		slog.Info("buildx get config", "error", err)
	}

	self.JsonResponseWithoutError(http, gin.H{
		"name":   builderName,
		"detail": detail,
		"config": config,
	})
	return
}
