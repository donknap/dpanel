package controller

import (
	"errors"
	"github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
	"os"
	"path/filepath"
	"sort"
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
		if item.Address == docker.Sdk.Client.DaemonHost() {
			currentName = item.Name
			break
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	self.JsonResponseWithoutError(http, gin.H{
		"currentName": currentName,
		"list":        result,
	})
	return
}

func (self Env) Create(http *gin.Context) {
	type ParamsValidate struct {
		Name      string `json:"name" binding:"required"`
		Title     string `json:"title" binding:"required"`
		Address   string `json:"address" binding:"required"`
		TlsCa     string `json:"tlsCa"`
		TlsCert   string `json:"tlsCert"`
		TlsKey    string `json:"tlsKey"`
		EnableTLS bool   `json:"enableTLS"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	if params.EnableTLS && (params.TlsCa == "" || params.TlsCert == "" || params.TlsKey == "") {
		self.JsonResponseWithError(http, errors.New("开启 TLS 时需要上传证书"), 500)
		return
	}
	options := docker.NewDockerClientOption{
		Host: params.Address,
	}
	if params.EnableTLS {
		certList := []struct {
			name    string
			content string
		}{
			{
				name:    "ca.pem",
				content: params.TlsCa,
			},
			{
				name:    "cert.pem",
				content: params.TlsCert,
			},
			{
				name:    "key.pem",
				content: params.TlsKey,
			},
		}
		certRootPath := filepath.Join("docker", params.Name)
		for _, s := range certList {
			path := filepath.Join(storage.Local{}.GetStorageCertPath(), certRootPath, s.name)
			err := os.MkdirAll(filepath.Dir(path), os.ModePerm)
			if err != nil {
				self.JsonResponseWithError(http, err, 500)
				return
			}
			err = os.WriteFile(path, []byte(s.content), 0o600)
			if err != nil {
				self.JsonResponseWithError(http, err, 500)
				return
			}
		}
		options.TlsCa = filepath.Join(certRootPath, "ca.pem")
		options.TlsCert = filepath.Join(certRootPath, "cert.pem")
		options.TlsKey = filepath.Join(certRootPath, "key.pem")
	}
	dockerClient, err := docker.NewDockerClient(options)
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
		Name:      params.Name,
		Title:     params.Title,
		Address:   params.Address,
		TlsCa:     options.TlsCa,
		TlsCert:   options.TlsCert,
		TlsKey:    options.TlsKey,
		EnableTLS: params.EnableTLS,
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

	options := docker.NewDockerClientOption{}
	address := ""
	if params.Name == "local" {
		address = ""
	} else {
		if row, ok := setting.Value.Docker[params.Name]; !ok {
			self.JsonResponseWithError(http, errors.New("Docker 客户端不存在，请先添加"), 500)
			return
		} else {
			address = row.Address
			if row.EnableTLS {
				options.TlsCa = row.TlsCa
				options.TlsCert = row.TlsCert
				options.TlsKey = row.TlsKey
			}
		}
	}
	options.Host = address
	if docker.Sdk.Client.DaemonHost() == address {
		self.JsonSuccessResponse(http)
		return
	}
	oldDockerClient := docker.Sdk
	dockerClient, _ := docker.NewDockerClient(options)
	_, err = dockerClient.Client.Info(dockerClient.Ctx)
	if err != nil {
		self.JsonResponseWithError(http, errors.New("Docker 客户端连接失败，请检查地址"), 500)
		return
	}
	oldDockerClient.CtxCancelFunc()
	oldDockerClient.Client.Close()

	docker.Sdk = dockerClient
	go logic.EventLogic{}.MonitorLoop()

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
			if docker.Sdk.Client.DaemonHost() == row.Address {
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
