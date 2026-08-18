package container

import (
	"errors"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/types/define"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
)

func New(dockerSdk *docker.Client, opts ...Option) (*Builder, error) {
	if dockerSdk == nil || dockerSdk.Client == nil {
		return nil, errors.New("docker sdk is empty")
	}
	var err error
	c := &Builder{
		dockerSdk: dockerSdk,
		containerConfig: &container.Config{
			ExposedPorts: make(nat.PortSet),
			Labels: map[string]string{
				"maintainer":             define.PanelAuthor,
				"com.dpanel.description": define.PanelDesc,
				"com.dpanel.website":     define.PanelWebSite,
				"com.dpanel.version":     facade.GetConfig().GetString("app.version"),
			},
		},
		hostConfig: &container.HostConfig{
			PortBindings: make(nat.PortMap),
			NetworkMode:  "default",
		},
		platform: &v1.Platform{},
		networkingConfig: &network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{},
		},
	}
	for _, opt := range opts {
		err = opt(c)
		if err != nil {
			return nil, err
		}
	}
	return c, nil
}

type Builder struct {
	containerConfig  *container.Config
	hostConfig       *container.HostConfig
	networkingConfig *network.NetworkingConfig
	platform         *v1.Platform
	containerName    string
	dockerSdk        *docker.Client
	rollback         *containerRollback
}

// Execute 完成容器创建或替换；新容器创建成功即提交替换并按配置处理旧容器。
// 后续网络连接或启动失败由上层保留新容器供用户修正后重试，不回滚旧容器；因此即使返回错误，非空 ID 仍表示最终占用目标名称的容器。
func (self *Builder) Execute() (containerID string, err error) {
	containerName := strings.TrimPrefix(strings.TrimSpace(self.containerName), "/")
	if containerName == "" {
		return "", errors.New("container name is empty")
	}
	if self.rollback == nil {
		response, err := self.dockerSdk.Client.ContainerCreate(
			self.dockerSdk.Ctx,
			self.containerConfig,
			self.hostConfig,
			self.networkingConfig,
			self.platform,
			containerName,
		)
		return response.ID, err
	}
	self.rollback.platform = self.platform
	if err = self.rollback.prepare(self.dockerSdk, containerName); err != nil {
		containerID, restoreErr := self.rollback.restore(self.dockerSdk, "")
		return containerID, errors.Join(err, restoreErr)
	}

	response, err := self.dockerSdk.Client.ContainerCreate(
		self.dockerSdk.Ctx,
		self.containerConfig,
		self.hostConfig,
		self.networkingConfig,
		self.platform,
		containerName,
	)
	if err != nil {
		containerID, restoreErr := self.rollback.restore(self.dockerSdk, response.ID)
		return containerID, errors.Join(err, restoreErr)
	}
	if commitErr := self.rollback.commit(self.dockerSdk); commitErr != nil {
		containerID, restoreErr := self.rollback.restore(self.dockerSdk, response.ID)
		return containerID, errors.Join(commitErr, restoreErr)
	}
	return response.ID, nil
}

func (self *Builder) GetConfig() (*container.Config, *container.HostConfig, *network.NetworkingConfig) {
	return self.containerConfig, self.hostConfig, self.networkingConfig
}
