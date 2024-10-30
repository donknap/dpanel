package docker

import (
	"context"
	"fmt"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
	"github.com/donknap/dpanel/common/function"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"io"
	"log/slog"
	"os"
	"strings"
)

type ContainerCreateBuilder struct {
	containerConfig  *container.Config
	hostConfig       *container.HostConfig
	networkingConfig *network.NetworkingConfig
	platform         *v1.Platform
	containerName    string
	err              error
	ctx              context.Context
}

func (self *ContainerCreateBuilder) WithContainerName(name string) *ContainerCreateBuilder {
	self.containerConfig.Hostname = fmt.Sprintf("%s.pod.dpanel.local", name)
	self.containerName = name
	//  防止退出
	self.containerConfig.AttachStdin = true
	self.containerConfig.AttachStdout = true
	self.containerConfig.AttachStderr = true
	self.containerConfig.Tty = true
	return self
}

func (self *ContainerCreateBuilder) WithEnv(name string, value string) *ContainerCreateBuilder {
	self.containerConfig.Env = append(self.containerConfig.Env, fmt.Sprintf("%s=%s", name, value))
	return self
}

func (self *ContainerCreateBuilder) WithImage(imageName string, tryPullImage bool) {
	// 只尝试从 docker.io 拉取
	if tryPullImage {
		reader, err := Sdk.Client.ImagePull(Sdk.Ctx, imageName, image.PullOptions{})
		if err != nil {
			self.err = err
			return
		}
		_, err = io.Copy(os.Stdout, reader)
		if err != nil {
			self.err = err
			return
		}

	}
	self.containerConfig.Image = imageName
	return
}

func (self *ContainerCreateBuilder) WithRestart(restartType string) *ContainerCreateBuilder {
	self.hostConfig.RestartPolicy = container.RestartPolicy{}
	self.hostConfig.RestartPolicy.Name = Sdk.GetRestartPolicyByString(restartType)
	return self
}

func (self *ContainerCreateBuilder) WithPrivileged() *ContainerCreateBuilder {
	self.hostConfig.Privileged = true
	return self
}

// WithVolume 挂载宿主机目录
func (self *ContainerCreateBuilder) WithVolume(host string, container string, readOnly bool) *ContainerCreateBuilder {
	permission := "rw"
	if readOnly {
		permission = "ro"
	}
	self.hostConfig.Binds = append(self.hostConfig.Binds, fmt.Sprintf("%s:%s:%s", host, container, permission))
	return self
}

// WithContainerVolume 挂载某个容器的数据卷
func (self *ContainerCreateBuilder) WithContainerVolume(fromContainerMd5 string) {
	self.hostConfig.VolumesFrom = append(self.hostConfig.VolumesFrom, fromContainerMd5)
}

func (self *ContainerCreateBuilder) WithDefaultVolume(container string) {
	volumePath := fmt.Sprintf("%s.%s", self.containerName, strings.Join(strings.Split(container, "/"), "-"))
	// 为了兼容之前生成的没有前缀的存储
	_, err := Sdk.Client.VolumeInspect(Sdk.Ctx, volumePath)
	if err != nil {
		volumePath = "dpanel." + volumePath
	}
	self.hostConfig.Binds = append(self.hostConfig.Binds, fmt.Sprintf("%s:%s:rw", volumePath, container))
}

// WithPort 绑定端口
func (self *ContainerCreateBuilder) WithPort(hostip, host, container string) *ContainerCreateBuilder {
	var port nat.Port
	var err error
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
	self.hostConfig.PortBindings[port] = make([]nat.PortBinding, 0)
	self.hostConfig.PortBindings[port] = append(
		self.hostConfig.PortBindings[port], nat.PortBinding{HostIP: hostip, HostPort: host},
	)
	return self
}

func (self *ContainerCreateBuilder) PublishAllPorts() {
	self.hostConfig.PublishAllPorts = true
}

func (self *ContainerCreateBuilder) WithLink(name string, alise string) {
	// 关联网络时，重新退出加入
	err := Sdk.Client.NetworkDisconnect(Sdk.Ctx, self.containerName, name, true)
	if err != nil {
		slog.Debug("disconnect network", "name", self.containerName, "error", err.Error())
	}
	err = Sdk.Client.NetworkConnect(Sdk.Ctx, self.containerName, name, &network.EndpointSettings{
		Aliases: []string{
			alise,
		},
	})
	if err != nil {
		slog.Debug("join network", "name", self.containerName, "error", err.Error())
	}
}

func (self ContainerCreateBuilder) WithNetwork(name string, alise string) {
	self.networkingConfig.EndpointsConfig[name] = &network.EndpointSettings{
		NetworkID: name,
		Aliases: []string{
			alise,
		},
	}
}

func (self *ContainerCreateBuilder) WithAutoRemove() {
	self.hostConfig.AutoRemove = true
}

func (self *ContainerCreateBuilder) WithCpus(count float32) {
	self.hostConfig.NanoCPUs = int64(count * 1000000000)
}

func (self *ContainerCreateBuilder) WithMemory(count int) {
	self.hostConfig.Memory = int64(count) * 1024 * 1024
}

func (self *ContainerCreateBuilder) WithShmSize(size int64) {
	self.hostConfig.ShmSize = size
}

func (self *ContainerCreateBuilder) WithWorkDir(path string) {
	self.containerConfig.WorkingDir = path
}

func (self *ContainerCreateBuilder) WithUser(user string) {
	self.containerConfig.User = user
}

func (self *ContainerCreateBuilder) WithCommandStr(cmd string) {
	cmdArr := function.CommandSplit(cmd)
	self.containerConfig.Cmd = cmdArr
}

func (self *ContainerCreateBuilder) WithCommand(cmd []string) {
	self.containerConfig.Cmd = cmd
}

func (self *ContainerCreateBuilder) WithEntrypointStr(cmd string) {
	cmdArr := function.CommandSplit(cmd)
	self.containerConfig.Entrypoint = cmdArr
}

func (self *ContainerCreateBuilder) WithEntrypoint(cmd []string) {
	self.containerConfig.Entrypoint = cmd
}

func (self *ContainerCreateBuilder) WithPid(pid ...string) {
	pidStr := strings.Join(pid, ":")
	self.hostConfig.PidMode = container.PidMode(pidStr)
}

func (self *ContainerCreateBuilder) WithNetworkMode(mode container.NetworkMode) {
	self.hostConfig.NetworkMode = mode
}

func (self *ContainerCreateBuilder) WithLog(driver string, maxSize string, maxFile string) {
	self.hostConfig.LogConfig = container.LogConfig{
		Type:   driver,
		Config: make(map[string]string),
	}
	if maxSize != "" {
		self.hostConfig.LogConfig.Config["max-size"] = maxSize
	}
	if maxFile != "" {
		self.hostConfig.LogConfig.Config["max-file"] = maxFile
	}
}

func (self *ContainerCreateBuilder) WithDns(ip []string) {
	if len(ip) > 0 {
		self.hostConfig.DNS = ip
	}
}

func (self *ContainerCreateBuilder) WithLabel(name, value string) {
	self.containerConfig.Labels[name] = value
}

func (self *ContainerCreateBuilder) WithExtraHosts(name, value string) {
	if self.hostConfig.ExtraHosts == nil {
		self.hostConfig.ExtraHosts = make([]string, 0)
	}
	self.hostConfig.ExtraHosts = append(self.hostConfig.ExtraHosts, fmt.Sprintf("%s:%s", name, value))
}

func (self *ContainerCreateBuilder) CreateOwnerNetwork(option network.CreateOptions) error {
	// 利用Network关联容器
	// 每次创建自身网络时，先删除掉，最后再统一将关联和自身加入进来
	// 容器关联时必须采用 hostname 以保证容器可以访问
	selfNetwork, err := Sdk.Client.NetworkInspect(Sdk.Ctx, self.containerName, network.InspectOptions{})
	if err == nil {
		for _, item := range selfNetwork.Containers {
			err = Sdk.Client.NetworkDisconnect(Sdk.Ctx, self.containerName, item.Name, true)
		}
		if err != nil {
			return err
		}
		_ = Sdk.Client.NetworkRemove(Sdk.Ctx, self.containerName)
	}
	options := make(map[string]string)
	options["name"] = self.containerName

	myOption := network.CreateOptions{
		Driver:     "bridge",
		Options:    options,
		EnableIPv6: option.EnableIPv6,
		IPAM:       option.IPAM,
	}
	_, err = Sdk.Client.NetworkCreate(Sdk.Ctx, self.containerName, myOption)
	if err != nil {
		slog.Debug("create network", "name", self.containerName, err)
	}
	return err
}

func (self *ContainerCreateBuilder) Execute() (response container.CreateResponse, err error) {
	if self.err != nil {
		return response, self.err
	}
	return Sdk.Client.ContainerCreate(
		self.ctx,
		self.containerConfig,
		self.hostConfig,
		self.networkingConfig,
		self.platform,
		self.containerName,
	)
}
