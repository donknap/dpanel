package docker

import (
	"context"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"log/slog"
	"strings"
)

type ContainerCreateBuilder struct {
	containerConfig  *container.Config
	hostConfig       *container.HostConfig
	networkingConfig *network.NetworkingConfig
	platform         *v1.Platform
	containerName    string
	err              error
	dockerSdk        *client.Client
	ctx              context.Context
}

func (self *ContainerCreateBuilder) withSdk(sdk *client.Client) *ContainerCreateBuilder {
	self.dockerSdk = sdk
	return self
}

func (self *ContainerCreateBuilder) WithContainerName(name string) *ContainerCreateBuilder {
	self.containerConfig.Hostname = fmt.Sprintf("%s.pod.dpanel.local", name)
	self.containerName = name
	self.containerConfig.Labels = map[string]string{
		"BuildAuthor":  BuilderAuthor,
		"BuildDesc":    BuildDesc,
		"BuildWebSite": BuildWebSite,
		"buildVersion": BuildVersion,
	}
	return self
}

func (self *ContainerCreateBuilder) WithEnv(name string, value string) *ContainerCreateBuilder {
	self.containerConfig.Env = append(self.containerConfig.Env, fmt.Sprintf("%s=%s", name, value))
	return self
}

func (self *ContainerCreateBuilder) WithImage(image string) *ContainerCreateBuilder {
	self.containerConfig.Image = image
	return self
}

func (self *ContainerCreateBuilder) WithRestart(restartType string) *ContainerCreateBuilder {
	self.hostConfig.RestartPolicy = container.RestartPolicy{}
	self.hostConfig.RestartPolicy.Name = restartType
	return self
}

func (self *ContainerCreateBuilder) WithPrivileged() *ContainerCreateBuilder {
	self.hostConfig.Privileged = true
	return self
}

// 挂载宿主机目录
func (self *ContainerCreateBuilder) WithVolume(host string, container string, permission string) *ContainerCreateBuilder {
	//_, err := os.Stat(host)
	self.hostConfig.Binds = append(self.hostConfig.Binds, fmt.Sprintf("%s:%s:%s", host, container, permission))

	//if os.IsNotExist(err) {
	//	self.hostConfig.Binds = append(self.hostConfig.Binds, fmt.Sprintf("%s:%s", host, container))
	//} else {
	//	self.hostConfig.Mounts = append(self.hostConfig.Mounts, mount.Mount{
	//		Type:     mount.TypeBind,
	//		Source:   host,
	//		Target:   container,
	//		ReadOnly: false,
	//	})
	//}
	return self
}

func (self *ContainerCreateBuilder) WithDefaultVolume(container string) {
	volumePath := fmt.Sprintf("%s.%s", self.containerName, strings.Join(strings.Split(container, "/"), "-"))
	self.hostConfig.Binds = append(self.hostConfig.Binds, fmt.Sprintf("%s:%s:rw", volumePath, container))
}

// 绑定端口
func (self *ContainerCreateBuilder) WithPort(host string, container string) *ContainerCreateBuilder {
	var port nat.Port
	var err error
	hostIp := "0.0.0.0"
	if strings.Contains(container, "/") {
		portArr := strings.Split(container, "/")
		port, err = nat.NewPort(portArr[1], portArr[0])
	} else {
		port, err = nat.NewPort("tcp", container)
	}
	if err != nil {
		self.err = err
		return nil
	}
	self.containerConfig.ExposedPorts[port] = struct{}{}
	self.hostConfig.PortBindings[port] = make([]nat.PortBinding, 0, 1)
	self.hostConfig.PortBindings[port] = append(
		self.hostConfig.PortBindings[port], nat.PortBinding{HostIP: hostIp, HostPort: host},
	)
	return self
}

func (self *ContainerCreateBuilder) WithLink(name string, alise string) {
	// 利用Network关联容器

	//if strings.Contains(name, "::") {
	//	nameArr := strings.Split(name, "::")
	//	name = nameArr[1]
	//	alise = nameArr[1]
	//}
	//self.hostConfig.Links = append(self.hostConfig.Links, fmt.Sprintf("%s:%s", name, alise))
	options := make(map[string]string)
	options["name"] = self.containerName
	myNetwork, _ := Sdk.Client.NetworkCreate(Sdk.Ctx, self.containerName, types.NetworkCreate{
		CheckDuplicate: true,
		Driver:         "bridge",
		Options:        options,
	})
	slog.Debug("create network", "name", myNetwork.ID)
	Sdk.Client.NetworkConnect(Sdk.Ctx, self.containerName, name, &network.EndpointSettings{
		Aliases: []string{
			alise,
		},
	})
}

func (self *ContainerCreateBuilder) Execute() (response container.CreateResponse, err error) {
	if self.err != nil {
		return response, self.err
	}
	return self.dockerSdk.ContainerCreate(
		self.ctx,
		self.containerConfig,
		self.hostConfig,
		self.networkingConfig,
		self.platform,
		self.containerName,
	)
}
