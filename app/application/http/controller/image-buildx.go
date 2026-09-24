package controller

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/donknap/dpanel/app/application/logic"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/exec/local"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
)

type ImageBuildx struct {
	controller.Abstract
}

func (self ImageBuildx) GetDetail(http *gin.Context) {
	sdk, err := docker.NewClientWithUser(http)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	buildx := logic.ImageBuildx{}
	target, err := buildx.GetTarget(sdk)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	builderName := fmt.Sprintf(define.DockerBuilderName, sdk.Name)
	var detail string
	if target.Exists {
		if v, err := sdk.RunResult("buildx", "inspect", builderName); err == nil {
			detail = string(v)
		} else {
			status := "stopped"
			if target.Running {
				status = "running"
			}
			detail = fmt.Sprintf("Name: %s\nDriver: docker-container\nStatus: %s", builderName, status)
		}
	}
	var config string
	if target.Exists {
		config, err = buildx.ReadTargetConfig(sdk, target)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	} else {
		buildxConfig, err := buildx.ResolveConfig(sdk.Name)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		config, err = buildx.ConfigContent(buildxConfig)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}

	self.JsonResponseWithoutError(http, gin.H{
		"name":   builderName,
		"detail": detail,
		"config": config,
		"proxy":  target.Proxy,
		"exists": target.Exists,
	})
}

func (self ImageBuildx) Create(http *gin.Context) {
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

	buildx := logic.ImageBuildx{}
	buildxConfig, err := buildx.ResolveConfig(sdk.Name)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	buildxConfig.ConfigContent = params.Config
	if err := buildx.WriteConfig(buildxConfig); err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	builderName := fmt.Sprintf(define.DockerBuilderName, sdk.Name)
	contextName := fmt.Sprintf(define.DockerContextName, sdk.Name)
	description := fmt.Sprintf("Created by DPanel DO NOT DELETE!!! %s", function.Sha256Struct(sdk.DockerEnv))
	if result, err := local.QuickRun("docker context inspect", contextName); err == nil {
		if !strings.Contains(string(result), description) {
			if _, err := sdk.RunResult("context", "rm", contextName, "--force"); err != nil {
				self.JsonResponseWithError(http, err, 500)
				return
			}
		}
	}
	if _, err := local.QuickRun("docker context inspect", contextName); err != nil {
		cmd, err := sdk.Run("context", "create", contextName, "--description", description)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		if _, err = cmd.RunWithResult(); err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}

	if err := buildx.RemoveTarget(sdk, true); err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	if _, err := sdk.RunResult("buildx", "inspect", builderName); err == nil {
		if _, err := sdk.RunResult("buildx", "rm", builderName, "--force", "--keep-daemon", "--keep-state"); err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}
	createArgs := []string{
		"buildx", "create",
		"--name", builderName,
		"--driver", "docker-container",
		"--driver-opt", "network=host",
		"--buildkitd-config", buildxConfig.ConfigPath,
	}
	proxy := params.Proxy
	if proxy != "" {
		createArgs = append(createArgs,
			"--driver-opt", "env.HTTP_PROXY="+proxy,
			"--driver-opt", "env.HTTPS_PROXY="+proxy,
		)
	}
	createArgs = append(createArgs, "--bootstrap", contextName)
	if _, err = sdk.RunResult(createArgs...); err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	self.JsonSuccessResponse(http)
}

func (self ImageBuildx) Prune(http *gin.Context) {
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
	buildx := logic.ImageBuildx{}

	builderName := fmt.Sprintf(define.DockerBuilderName, sdk.Name)
	contextName := fmt.Sprintf(define.DockerContextName, sdk.Name)
	if params.EnablePrune {
		if err := buildx.PruneTarget(sdk); err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}
	if params.EnableRemove {
		// 旧版请求没有 enableForceRemove 字段，默认保持原有的强制删除行为。
		forceRemove := params.EnableForceRemove == nil || *params.EnableForceRemove
		if err := buildx.RemoveTarget(sdk, forceRemove); err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		if _, err := sdk.RunResult("buildx", "inspect", builderName); err == nil {
			if _, err := sdk.RunResult("buildx", "rm", builderName, "--force", "--keep-daemon", "--keep-state"); err != nil {
				self.JsonResponseWithError(http, err, 500)
				return
			}
		}
		if _, err := local.QuickRun("docker context inspect", contextName); err == nil {
			removeContextArgs := []string{"context", "rm", contextName}
			_, removeErr := sdk.RunResult(removeContextArgs...)
			if removeErr != nil && forceRemove {
				removeContextArgs = append(removeContextArgs, "--force")
				_, removeErr = sdk.RunResult(removeContextArgs...)
			}
			if removeErr != nil {
				self.JsonResponseWithError(http, removeErr, 500)
				return
			}
			if _, err := local.QuickRun("docker context inspect", contextName); err == nil {
				self.JsonResponseWithError(http, fmt.Errorf("docker context %s still exists after removal", contextName), 500)
				return
			}
		}
		buildxConfigRoot := filepath.Join(storage.Local{}.GetStorageLocalPath(), "buildx")
		if err := function.SafeDeleteAll(buildxConfigRoot, sdk.Name); err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}
	self.JsonSuccessResponse(http)
}
