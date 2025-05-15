package controller

import (
	"errors"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/ssh"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/event"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
	"log/slog"
	"net/url"
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
		docker.Client
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	if !self.Validate(http, &params) {
		return
	}
	if params.EnableTLS && (params.TlsCa == "" || params.TlsCert == "" || params.TlsKey == "") {
		self.JsonResponseWithError(http, errors.New("开启 TLS 时需要上传证书"), 500)
		return
	}

	if params.EnableSSH {
		urls, err := url.Parse(params.Address)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		if urls.Scheme == "unix" {
			params.SshServerInfo.Host = "127.0.0.1"
		} else {
			params.SshServerInfo.Host = urls.Hostname()
		}
		if params.SshServerInfo.Host == "" {
			params.SshServerInfo.Host = params.Address
		}
		//if urls.Hostname() == "172.16.1.13" {
		//	params.SshServerInfo.Host = "172.16.1.148"
		//}
		sshClient, err := ssh.NewClient(ssh.WithServerInfo(params.SshServerInfo)...)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		defer func() {
			sshClient.Close()
		}()
		result, err := sshClient.Run("pwd")
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		slog.Debug("docker env", "ssh home", result)
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

	if params.RemoteType == "ssh" {
		options = append(options, docker.WithSSH(params.SshServerInfo))
	}

	client := &docker.Client{
		Name:              params.Name,
		Title:             params.Title,
		Address:           params.Address,
		Default:           defaultEnv,
		ServerUrl:         params.ServerUrl,
		EnableTLS:         params.EnableTLS,
		TlsCa:             params.TlsCa,
		TlsCert:           params.TlsCert,
		TlsKey:            params.TlsKey,
		EnableComposePath: params.EnableComposePath,
		ComposePath:       params.ComposePath,
		EnableSSH:         params.EnableSSH,
		SshServerInfo:     params.SshServerInfo,
		RemoteType:        params.RemoteType,
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
	info, err := dockerClient.Client.Info(dockerClient.Ctx)
	if err != nil {
		dockerClient.Close()
		self.JsonResponseWithError(http, errors.New("Docker 客户端连接失败，错误信息："+err.Error()), 500)
		return
	}
	fmt.Printf("%v \n", info.Name)
	if defaultEnv {
		// 获取面板信息
		if info, err := dockerClient.Client.ContainerInspect(dockerClient.Ctx, facade.GetConfig().GetString("app.name")); err == nil {
			_ = logic.Setting{}.Save(&entity.Setting{
				GroupName: logic.SettingGroupSetting,
				Name:      logic.SettingGroupSettingDPanelInfo,
				Value: &accessor.SettingValueOption{
					DPanelInfo: &info,
				},
			})
		}
		if defaultDockerInfo, err := dockerClient.Client.Info(dockerClient.Ctx); err == nil {
			client.DockerInfo = &docker.ClientDockerInfo{
				Name: defaultDockerInfo.Name,
				ID:   defaultDockerInfo.ID,
			}
		}
	}

	logic.DockerEnv{}.UpdateEnv(client)
	// 如果修改的是当前客户端的连接地址，则更新 docker sdk
	if docker.Sdk.Name == params.Name && docker.Sdk.Client.DaemonHost() != params.Address {
		docker.Sdk.Close()
		docker.Sdk = dockerClient
	} else {
		dockerClient.Close()
	}
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

	dockerEnv, err := logic.DockerEnv{}.GetEnvByName(params.Name)
	if err != nil {
		self.JsonResponseWithError(http, function.ErrorMessage(".commonDataNotFoundOrDeleted"), 500)
		return
	}
	options := make([]docker.Option, 0)
	if docker.Sdk.Client.DaemonHost() == dockerEnv.Address {
		self.JsonSuccessResponse(http)
		return
	}
	options = append(options, docker.WithAddress(dockerEnv.Address))
	options = append(options, docker.WithName(dockerEnv.Name))
	if dockerEnv.EnableTLS {
		options = append(options, docker.WithTLS(dockerEnv.TlsCa, dockerEnv.TlsCert, dockerEnv.TlsKey))
	}
	if dockerEnv.RemoteType == docker.RemoteTypeSSH {
		options = append(options, docker.WithSSH(dockerEnv.SshServerInfo))
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
	if dockerEnv.RemoteType == docker.RemoteTypeDocker {
		go logic.EventLogic{}.MonitorLoop()
	}

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

			facade.GetEvent().Publish(event.EnvDeleteEvent, event.EnvPayload{
				Name: name,
				Ctx:  http,
			})
		}
	}
	_ = logic.Setting{}.Save(setting)

	self.JsonSuccessResponse(http)
	return
}

func (self Env) GetDetail(http *gin.Context) {
	type ParamsValidate struct {
		Name string `json:"name"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	if params.Name == "" {
		params.Name = docker.Sdk.Name
	}

	dockerEnv, err := logic.DockerEnv{}.GetEnvByName(params.Name)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	if dockerEnv.ServerUrl == "" {
		if d, err := url.Parse(dockerEnv.Address); err == nil {
			if b, _, exists := strings.Cut(d.Host, ":"); exists {
				dockerEnv.ServerUrl = b
			}
		}
	}
	if dockerEnv.ServerUrl == "" {
		dockerEnv.ServerUrl = "127.0.0.1"
	}
	self.JsonResponseWithoutError(http, dockerEnv)
	return
}
