package controller

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	types2 "github.com/donknap/dpanel/common/service/docker/types"
	event2 "github.com/donknap/dpanel/common/service/notice"
	"github.com/donknap/dpanel/common/service/ssh"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/donknap/dpanel/common/types/event"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
)

type Env struct {
	controller.Abstract
}

func (self Env) GetList(http *gin.Context) {
	type ParamsValidate struct {
		EnableCertContent bool `json:"enableCertContent"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	result := make([]*types2.DockerEnv, 0)
	if setting, err := (logic.Setting{}).GetValue(logic.SettingGroupSetting, logic.SettingGroupSettingDocker); err == nil {
		for _, item := range setting.Value.Docker {
			if params.EnableCertContent && item.EnableTLS {
				if content, err := os.ReadFile(filepath.Join(storage.Local{}.GetCertPath(), item.TlsCa)); err == nil {
					item.TlsCa = string(content)
				} else {
					item.TlsCa = ""
				}
				if content, err := os.ReadFile(filepath.Join(storage.Local{}.GetCertPath(), item.TlsCert)); err == nil {
					item.TlsCert = string(content)
				} else {
					item.TlsCert = ""
				}
				if content, err := os.ReadFile(filepath.Join(storage.Local{}.GetCertPath(), item.TlsKey)); err == nil {
					item.TlsKey = string(content)
				} else {
					item.TlsKey = ""
				}
			}
			result = append(result, item)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	self.JsonResponseWithoutError(http, gin.H{
		"currentName": docker.Sdk.Name,
		"list": function.PluckArrayWalk(result, func(item *types2.DockerEnv) (*types2.DockerEnv, bool) {
			status := types2.DockerStatus{
				Available: true,
				Message:   "",
			}
			if v, ok := storage.Cache.Get(fmt.Sprintf(storage.CacheKeyDockerStatus, item.Name)); ok {
				status = v.(types2.DockerStatus)
			}
			item.DockerStatus = &status
			if item.SshServerInfo != nil && item.SshServerInfo.Password != "" {
				item.SshServerInfo.Password = "******"
			}
			return item, true
		}),
	})
	return
}

func (self Env) Create(http *gin.Context) {
	type ParamsValidate struct {
		types2.DockerEnv
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	if !self.Validate(http, &params) {
		return
	}
	if params.EnableTLS && (params.TlsCa == "" || params.TlsCert == "" || params.TlsKey == "") {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageSystemEnvTlsInvalidCert), 500)
		return
	}
	var oldDockerEnv *types2.DockerEnv
	if v, err := (logic.Env{}).GetEnvByName(params.Name); err == nil {
		oldDockerEnv = v
		if params.SshServerInfo != nil && params.SshServerInfo.Password == "******" {
			params.SshServerInfo.Password = oldDockerEnv.SshServerInfo.Password
		}
	}

	if params.EnableSSH {
		knownHostsCallback := ssh.NewDefaultKnownHostCallback()
		if params.SshServerInfo != nil && params.SshServerInfo.Address != "" {
			_ = knownHostsCallback.Delete(params.SshServerInfo.Address, params.SshServerInfo.Port)
		}
		sshClient, err := ssh.NewClient(ssh.WithServerInfo(params.SshServerInfo)...)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		defer func() {
			sshClient.Close()
		}()
		// ssh 密码加密
		if v, err := function.RSAEncode(params.SshServerInfo.Password); err == nil {
			params.SshServerInfo.Password = v
		}
	}

	defaultEnv := false
	if params.Name == "local" {
		defaultEnv = true
	}

	if params.EnableComposePath {
		if params.ComposePath == "" {
			params.ComposePath = fmt.Sprintf("compose-%s", params.Name)
		}
	}
	_ = os.MkdirAll(filepath.Join(storage.Local{}.GetStorageLocalPath(), params.ComposePath), 0755)

	dockerEnv := &types2.DockerEnv{
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
		DockerType:        params.DockerType,
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
		for _, s := range certList {
			if s.content == "" {
				continue
			}
			path := filepath.Join(storage.Local{}.GetCertPath(), dockerEnv.CertRoot(), s.name)
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
		dockerEnv.TlsCa = filepath.Join(dockerEnv.CertRoot(), "ca.pem")
		dockerEnv.TlsCert = filepath.Join(dockerEnv.CertRoot(), "cert.pem")
		dockerEnv.TlsKey = filepath.Join(dockerEnv.CertRoot(), "key.pem")
	}

	dockerClient, err := docker.NewClientWithDockerEnv(dockerEnv, docker.WithSockProxy())
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	_, err = dockerClient.Client.Info(dockerClient.GetTryCtx())
	if err != nil {
		dockerClient.Close()
		if function.ErrorHasKeyword(err, "Maximum supported") {
			self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageSystemEnvApiTooOld, "err", err.Error()), 500)
			return
		}
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageSystemEnvDockerApiFailed, "error", err.Error()), 500)
		return
	}
	logic.Env{}.UpdateEnv(dockerEnv)
	event2.Monitor.Join(dockerEnv)

	// 如果修改的是当前客户端的连接地址，则更新 docker sdk
	if docker.Sdk.Name == params.Name {
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

	dockerEnv, err := logic.Env{}.GetEnvByName(params.Name)
	if err != nil {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted), 500)
		return
	}
	oldDockerClient := docker.Sdk
	dockerClient, err := docker.NewClientWithDockerEnv(dockerEnv, docker.WithSockProxy())
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	_, err = dockerClient.Client.Info(dockerClient.GetTryCtx())
	if err != nil {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageSystemEnvDockerApiFailed, "error", err.Error()), 500)
		return
	}
	oldDockerClient.CtxCancelFunc()
	err = oldDockerClient.Client.Close()
	if err != nil {
		slog.Debug("env switch close old", "error", err)
	}
	docker.Sdk = dockerClient

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
			self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted), 500)
			return
		} else {
			if docker.Sdk.DockerEnv.Name == row.Name {
				self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageSystemEnvCurrentCanNotDelete), 500)
				return
			}
			delete(setting.Value.Docker, name)

			event2.Monitor.Leave(name)

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

	dockerEnv, err := logic.Env{}.GetEnvByName(params.Name)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	if dockerEnv.ServerUrl == "" {
		if d, err := url.Parse(dockerEnv.Address); err == nil {
			if d.Host == "" && d.Scheme == "" {
				dockerEnv.ServerUrl = d.Path
			} else if b, _, exists := strings.Cut(d.Host, ":"); d.Host != "" && exists {
				dockerEnv.ServerUrl = b
			}
		}
	}
	if dockerEnv.EnableSSH && dockerEnv.Name != define.DockerDefaultClientName && dockerEnv.ServerUrl == "" {
		dockerEnv.ServerUrl = dockerEnv.SshServerInfo.Address
	}
	if dockerEnv.ServerUrl == "" {
		dockerEnv.ServerUrl = "127.0.0.1"
	}
	self.JsonResponseWithoutError(http, dockerEnv)
	return
}
