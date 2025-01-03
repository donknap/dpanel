package controller

import (
	"errors"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Env struct {
	controller.Abstract
}

func (self Env) GetList(http *gin.Context) {
	result := make([]*docker.Client, 0)

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
		Name              string `json:"name" binding:"required"`
		Title             string `json:"title" binding:"required"`
		Address           string `json:"address" binding:"required"`
		TlsCa             string `json:"tlsCa"`
		TlsCert           string `json:"tlsCert"`
		TlsKey            string `json:"tlsKey"`
		EnableTLS         bool   `json:"enableTLS"`
		EnableComposePath bool   `json:"enableComposePath"`
		ComposePath       string `json:"composePath"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	if params.EnableTLS && (params.TlsCa == "" || params.TlsCert == "" || params.TlsKey == "") {
		self.JsonResponseWithError(http, errors.New("开启 TLS 时需要上传证书"), 500)
		return
	}
	defaultEnv := false
	if params.Name == "local" {
		defaultEnv = true
	}
	options := []docker.Option{
		docker.WithAddress(params.Address),
		docker.WithName(params.Name),
	}
	if params.EnableComposePath {
		if params.ComposePath == "" {
			params.ComposePath = fmt.Sprintf("compose-%s", params.Name)
		}
	}
	_ = os.MkdirAll(filepath.Join(storage.Local{}.GetStorageLocalPath(), params.ComposePath), 0755)

	client := &docker.Client{
		Name:              params.Name,
		Title:             params.Title,
		Address:           params.Address,
		TlsCa:             params.TlsCa,
		TlsCert:           params.TlsCert,
		TlsKey:            params.TlsKey,
		EnableTLS:         params.EnableTLS,
		Default:           defaultEnv,
		EnableComposePath: params.EnableComposePath,
		ComposePath:       params.ComposePath,
	}

	if params.EnableTLS {
		if strings.HasSuffix(params.TlsCa, ".pem") {
			options = append(options, docker.WithTLS(params.TlsCa, params.TlsCert, params.TlsKey))
		} else {
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
			client.TlsCa = filepath.Join(certRootPath, "ca.pem")
			client.TlsCert = filepath.Join(certRootPath, "cert.pem")
			client.TlsKey = filepath.Join(certRootPath, "key.pem")

			options = append(options,
				docker.WithTLS(
					client.TlsCa,
					client.TlsCert,
					client.TlsKey,
				),
			)
		}
	}
	dockerClient, err := docker.NewBuilder(options...)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	defer func() {
		dockerClient.CtxCancelFunc()
		_ = dockerClient.Client.Close()
	}()
	_, err = dockerClient.Client.Info(docker.Sdk.Ctx)
	if err != nil {
		self.JsonResponseWithError(http, errors.New("Docker 客户端连接失败，错误信息："+err.Error()), 500)
		return
	}
	logic.DockerEnv{}.UpdateEnv(client)
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
	options := make([]docker.Option, 0)
	if row, ok := setting.Value.Docker[params.Name]; !ok {
		self.JsonResponseWithError(http, errors.New("Docker 客户端不存在，请先添加"), 500)
		return
	} else {
		if docker.Sdk.Client.DaemonHost() == row.Address {
			self.JsonSuccessResponse(http)
			return
		}
		options = append(options, docker.WithAddress(row.Address))
		options = append(options, docker.WithName(row.Name))
		if row.EnableTLS {
			options = append(options, docker.WithTLS(row.TlsCa, row.TlsCert, row.TlsKey))
		}
	}
	oldDockerClient := docker.Sdk

	dockerClient, err := docker.NewBuilder(options...)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	_, err = dockerClient.Client.Info(dockerClient.Ctx)
	if err != nil {
		self.JsonResponseWithError(http, errors.New("Docker 客户端连接失败，请检查地址"), 500)
		return
	}
	oldDockerClient.CtxCancelFunc()
	_ = oldDockerClient.Client.Close()

	docker.Sdk = dockerClient
	go logic.EventLogic{}.MonitorLoop()

	// 清除掉统计数据
	_ = logic.Setting{}.Save(&entity.Setting{
		GroupName: logic.SettingGroupSetting,
		Name:      logic.SettingGroupSettingDiskUsage,
		Value: &accessor.SettingValueOption{
			DiskUsage: &accessor.DiskUsage{
				Usage:     &types.DiskUsage{},
				UpdatedAt: time.Now(),
			},
		},
	})
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
