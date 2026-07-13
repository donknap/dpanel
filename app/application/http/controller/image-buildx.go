package controller

import (
	"fmt"
	"log/slog"
	"os"
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
	imageLogic := logic.Image{}
	createConfigOption, err := imageLogic.BuildxConfig(docker.Sdk.Name)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	builderName := fmt.Sprintf(define.DockerBuilderName, docker.Sdk.Name)
	var detail string
	if v, err := docker.Sdk.RunResult("buildx", "inspect", builderName); err == nil {
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
	proxyPath := filepath.Join(filepath.Dir(createConfigOption.ConfigPath), "proxy")
	var proxy string
	proxyBytes, err := os.ReadFile(proxyPath)
	if err == nil {
		proxy = string(proxyBytes)
	} else {
		proxy = os.Getenv("HTTP_PROXY")
		if proxy == "" {
			proxy = os.Getenv("HTTPS_PROXY")
		}
	}

	self.JsonResponseWithoutError(http, gin.H{
		"name":   builderName,
		"detail": detail,
		"config": config,
		"proxy":  proxy,
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

	imageLogic := logic.Image{}
	createConfigOption, err := imageLogic.BuildxConfig(docker.Sdk.Name)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	if params.Config != nil {
		createConfigOption.ConfigContent = params.Config
		if err := imageLogic.BuildxCreateConfig(createConfigOption); err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	} else if _, err := os.Stat(createConfigOption.ConfigPath); os.IsNotExist(err) {
		if err := imageLogic.BuildxCreateConfig(createConfigOption); err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	} else if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	proxyPath := filepath.Join(filepath.Dir(createConfigOption.ConfigPath), "proxy")
	if err := os.WriteFile(proxyPath, []byte(params.Proxy), 0600); err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	builderName := fmt.Sprintf(define.DockerBuilderName, docker.Sdk.Name)
	contextName := fmt.Sprintf(define.DockerContextName, docker.Sdk.Name)
	description := fmt.Sprintf("Created by DPanel DO NOT DELETE!!! %s", function.Sha256Struct(docker.Sdk.DockerEnv))
	if result, err := local.QuickRun("docker context inspect", contextName); err == nil {
		if !strings.Contains(string(result), description) {
			if _, err := docker.Sdk.RunResult("context", "rm", contextName, "--force"); err != nil {
				self.JsonResponseWithError(http, err, 500)
				return
			}
		}
	}
	if _, err := local.QuickRun("docker context inspect", contextName); err != nil {
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

	if _, err := docker.Sdk.RunResult("buildx", "rm", builderName, "--force"); err != nil {
		slog.Debug("image build rm buildx", "error", err)
	}
	createArgs := []string{
		"buildx", "create",
		"--name", builderName,
		"--driver", "docker-container",
		"--driver-opt", "network=host",
		"--buildkitd-config", createConfigOption.ConfigPath,
	}
	proxy := params.Proxy
	if proxy != "" {
		createArgs = append(createArgs,
			"--driver-opt", "env.HTTP_PROXY="+proxy,
			"--driver-opt", "env.HTTPS_PROXY="+proxy,
		)
	}
	if noProxy := os.Getenv("NO_PROXY"); noProxy != "" {
		createArgs = append(createArgs, "--driver-opt", "env.NO_PROXY="+noProxy)
	}
	createArgs = append(createArgs, "--bootstrap", contextName)
	if _, err = docker.Sdk.RunResult(createArgs...); err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	self.JsonSuccessResponse(http)
}

func (self ImageBuildx) Prune(http *gin.Context) {
	type ParamsValidate struct {
		EnablePrune  bool `json:"enablePrune"`
		EnableRemove bool `json:"enableRemove"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	builderName := fmt.Sprintf(define.DockerBuilderName, docker.Sdk.Name)
	contextName := fmt.Sprintf(define.DockerContextName, docker.Sdk.Name)
	if params.EnablePrune {
		if _, err := docker.Sdk.RunResult("buildx", "prune", "--builder", builderName, "--all", "--force"); err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}
	if params.EnableRemove {
		if _, err := docker.Sdk.RunResult("buildx", "rm", builderName, "--force"); err != nil {
			slog.Debug("image buildx delete builder", "error", err)
		}
		if _, err := local.QuickRun("docker context inspect", contextName); err == nil {
			if _, err := docker.Sdk.RunResult("context", "rm", contextName, "--force"); err != nil {
				self.JsonResponseWithError(http, err, 500)
				return
			}
		}
		buildxConfigRoot := filepath.Join(storage.Local{}.GetStorageLocalPath(), "buildx")
		if err := function.SafeDeleteAll(buildxConfigRoot, docker.Sdk.Name); err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}
	self.JsonSuccessResponse(http)
}
