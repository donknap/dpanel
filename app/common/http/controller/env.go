package controller

import (
	"errors"
	"fmt"
	"github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
	"runtime"
)

type Env struct {
	controller.Abstract
}

func (self Env) GetList(http *gin.Context) {
	result := make([]*accessor.DockerClientResult, 0)

	setting, err := logic.Setting{}.GetValue(logic.SettingGroupSetting, logic.SettingGroupSettingDocker)
	if err == nil {
		for _, item := range setting.Value.Docker {
			result = append(result, item)
		}
	}

	currentName := "local"
	for _, item := range result {
		if item.Address == docker.Sdk.Address {
			currentName = item.Name
			break
		}
	}
	self.JsonResponseWithoutError(http, gin.H{
		"currentName": currentName,
		"list":        result,
	})
	return
}

func (self Env) Create(http *gin.Context) {
	type ParamsValidate struct {
		Name    string `json:"name" binding:"required"`
		Title   string `json:"title" binding:"required"`
		Address string `json:"address" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	dockerClient, err := docker.NewDockerClient(params.Address)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	_, err = dockerClient.Client.Info(docker.Sdk.Ctx)
	if err != nil {
		self.JsonResponseWithError(http, errors.New("Docker 客户端连接失败，请检查地址"), 500)
		return
	}
	logic.DockerEnv{}.UpdateEnv(&accessor.DockerClientResult{
		Name:    params.Name,
		Title:   params.Title,
		Address: params.Address,
	})

	self.JsonSuccessResponse(http)
	return
}

func (self Env) Switch(http *gin.Context) {
	type ParamsValidate struct {
		Name string `json:"name" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	setting, err := logic.Setting{}.GetValue(logic.SettingGroupSetting, logic.SettingGroupSettingDocker)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	address := ""
	if params.Name == "local" {
		address = ""
	} else {
		if row, ok := setting.Value.Docker[params.Name]; !ok {
			self.JsonResponseWithError(http, errors.New("Docker 客户端不存在，请先添加"), 500)
			return
		} else {
			address = row.Address
		}
	}
	fmt.Printf("%v \n", address)
	fmt.Printf("%v \n", runtime.NumGoroutine())

	if docker.Sdk.Address == address {
		self.JsonSuccessResponse(http)
		return
	}
	oldDockerClient := docker.Sdk
	defer func() {
		oldDockerClient.CtxCancelFunc()
		oldDockerClient.Client.Close()
	}()

	dockerClient, _ := docker.NewDockerClient(address)
	_, err = dockerClient.Client.Info(dockerClient.Ctx)
	if err != nil {
		self.JsonResponseWithError(http, errors.New("Docker 客户端连接失败，请检查地址"), 500)
		return
	}
	docker.Sdk = dockerClient
	go logic.EventLogic{}.MonitorLoop()

	fmt.Printf("%v \n", runtime.NumGoroutine())
	self.JsonSuccessResponse(http)
	return
}

func (self Env) Delete(http *gin.Context) {
	type ParamsValidate struct {
		Name []string `json:"name" binding:"required"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	setting, err := logic.Setting{}.GetValue(logic.SettingGroupSetting, logic.SettingGroupSettingDocker)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	for _, name := range params.Name {
		if row, ok := setting.Value.Docker[name]; !ok {
			self.JsonResponseWithError(http, errors.New("Docker 客户端不存在，请先添加"), 500)
			return
		} else {
			if docker.Sdk.Address == row.Address {
				docker.Sdk.CtxCancelFunc()
				_ = docker.Sdk.Client.Close()
			}
			delete(setting.Value.Docker, name)
		}
	}
	_ = logic.Setting{}.Save(setting)
	self.JsonSuccessResponse(http)
	return
}
