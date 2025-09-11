package controller

import (
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/exec/local"
	"github.com/donknap/dpanel/common/service/exec/remote"
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

	result := make([]*docker.Client, 0)
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
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageSystemEnvTlsInvalidCert), 500)
		return
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
		homeDir, err := remote.QuickRun(sshClient, "echo $HOME")
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		slog.Debug("docker env", "ssh home", string(homeDir))

		if params.RemoteType == docker.RemoteTypeSSH && params.SshServerInfo.AuthType != ssh.SshAuthTypePemDefault {
			publicKey, _, err := storage.GetCertRsaContent()
			if err != nil {
				self.JsonResponseWithError(http, err, 500)
				return
			}
			sftp, err := sshClient.NewSftpSession()
			if err != nil {
				self.JsonResponseWithError(http, err, 500)
				return
			}
			defer func() {
				_ = sftp.Close()
			}()
			// 这里不能使用 filepath 可能会造成运行环境与服务器路径不一致
			authKeyFile := fmt.Sprintf("%s/.ssh/authorized_keys", string(homeDir))
			err = sftp.MkdirAll(fmt.Sprintf("%s/.ssh", string(homeDir)))
			if err != nil {
				self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageSystemEnvDockerCreateSSHHomeDirFailed, "user", params.SshServerInfo.Username, "error", err.Error()), 500)
				return
			}
			file, err := sftp.OpenFile(authKeyFile, os.O_CREATE|os.O_RDWR|os.O_APPEND)
			if err != nil {
				self.JsonResponseWithError(http, err, 500)
				return
			}
			defer func() {
				_ = file.Close()
			}()
			content, err := io.ReadAll(file)
			if err != nil {
				self.JsonResponseWithError(http, err, 500)
				return
			}
			if !strings.Contains(string(content), string(publicKey)) {
				_, err = file.Write(publicKey)
				if err != nil {
					self.JsonResponseWithError(http, err, 500)
					return
				}
			}
		}
		// 验证是否成功配置证书可以正常连接
		if params.RemoteType == docker.RemoteTypeSSH {
			result, err := local.QuickRun(fmt.Sprintf("ssh %s@%s -p %d pwd", params.SshServerInfo.Username, params.SshServerInfo.Address, params.SshServerInfo.Port))
			if err != nil {
				slog.Debug("docker env docker -H ssh://", "error", string(result))
				self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageSystemEnvDockerApiSSHFailed), 500)
				return
			}
		}
		// 如果当前是 windows 且使用的 wsl 中的 docker cli 还需要同步 .ssh 目录
		if runtime.GOOS == "windows" && docker.CliInWSL() {
			osUser, _ := user.Current()
			if result, err := local.QuickRun(fmt.Sprintf("wsl cp -r /mnt/c/Users/%s/.ssh/* ~/.ssh/ && chmod 600 ~/.ssh/id*", filepath.Base(osUser.HomeDir))); err != nil {
				slog.Debug("docker env copy id_rsa to wsl", "error", string(result))
				self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageSystemEnvDockerApiSSHFailed), 500)
				return
			}
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
			path := filepath.Join(storage.Local{}.GetCertPath(), client.CertRoot(), s.name)
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
		client.TlsCa = filepath.Join(client.CertRoot(), "ca.pem")
		client.TlsCert = filepath.Join(client.CertRoot(), "cert.pem")
		client.TlsKey = filepath.Join(client.CertRoot(), "key.pem")
	}

	dockerClient, err := docker.NewBuilderWithDockerEnv(client)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	_, err = dockerClient.Client.Info(dockerClient.Ctx)
	if err != nil {
		dockerClient.Close()
		if function.ErrorHasKeyword(err, "Maximum supported") {
			self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageSystemEnvApiTooOld, "err", err.Error()), 500)
			return
		}
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageSystemEnvDockerApiFailed, "error", err.Error()), 500)
		return
	}

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
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted), 500)
		return
	}
	if docker.Sdk.Name == dockerEnv.Name {
		self.JsonSuccessResponse(http)
		return
	}
	oldDockerClient := docker.Sdk
	dockerClient, err := docker.NewBuilderWithDockerEnv(dockerEnv)
	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}
	_, err = dockerClient.Client.Info(dockerClient.Ctx)
	if err != nil {
		self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageSystemEnvDockerApiFailed, "error", err.Error()), 500)
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
			self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted), 500)
			return
		} else {
			if docker.Sdk.Client.DaemonHost() == row.Address {
				self.JsonResponseWithError(http, function.ErrorMessage(define.ErrorMessageSystemEnvCurrentCanNotDelete), 500)
				return
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
			if d.Host == "" && d.Scheme == "" {
				dockerEnv.ServerUrl = d.Path
			} else if b, _, exists := strings.Cut(d.Host, ":"); d.Host != "" && exists {
				dockerEnv.ServerUrl = b
			}
		}
	}
	if dockerEnv.EnableSSH && dockerEnv.ServerUrl == "" {
		dockerEnv.ServerUrl = dockerEnv.SshServerInfo.Address
	}
	if dockerEnv.ServerUrl == "" {
		dockerEnv.ServerUrl = "127.0.0.1"
	}
	self.JsonResponseWithoutError(http, dockerEnv)
	return
}
