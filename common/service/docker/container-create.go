package docker

import (
	"bufio"
	"context"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"io"
)

type ContainerCreateBuilder struct {
	containerConfig  *container.Config
	hostConfig       *container.HostConfig
	networkingConfig *network.NetworkingConfig
	platform         *v1.Platform
	containerName    string
	err              error
	dockerSdk        *client.Client
}

func (self *ContainerCreateBuilder) withSdk(sdk *client.Client) *ContainerCreateBuilder {
	self.dockerSdk = sdk
	return self
}

func (self *ContainerCreateBuilder) WithContainerName(name string) *ContainerCreateBuilder {
	self.containerConfig.Hostname = fmt.Sprintf("%s.pod.dpanel.local", name)
	self.containerName = name
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

func (self *ContainerCreateBuilder) WithAlwaysRestart() *ContainerCreateBuilder {
	self.hostConfig.RestartPolicy = container.RestartPolicy{}
	self.hostConfig.RestartPolicy.IsAlways()
	return self
}

func (self *ContainerCreateBuilder) WithPrivileged() *ContainerCreateBuilder {
	self.hostConfig.Privileged = true
	return self
}

// 挂载宿主机目录
func (self *ContainerCreateBuilder) WithVolume(host string, container string) *ContainerCreateBuilder {
	self.hostConfig.Binds = append(self.hostConfig.Binds, fmt.Sprintf("%s:%s", host, container))
	return self
}

// 绑定端口
func (self *ContainerCreateBuilder) WithPort(host string, container string) *ContainerCreateBuilder {
	port, err := nat.NewPort("tcp", container)
	if err != nil {
		self.err = err
		return nil
	}
	self.containerConfig.ExposedPorts[port] = struct{}{}
	self.hostConfig.PortBindings[port] = make([]nat.PortBinding, 0, 1)
	self.hostConfig.PortBindings[port] = append(
		self.hostConfig.PortBindings[port], nat.PortBinding{HostIP: "0.0.0.0", HostPort: host},
	)
	return self
}

func (self *ContainerCreateBuilder) WithLink(name string, alise string) *ContainerCreateBuilder {
	self.hostConfig.Links = append(self.hostConfig.Links, fmt.Sprintf("%s:%s", name, alise))
	return self
}

func (self *ContainerCreateBuilder) Execute() (response container.CreateResponse, err error) {
	if self.err != nil {
		return response, self.err
	}
	ctx := context.Background()
	reader, err := self.dockerSdk.ImagePull(ctx, self.containerConfig.Image, types.ImagePullOptions{})
	if err != nil {
		return response, err
	}
	defer reader.Close()

	out := bufio.NewReader(reader)
	for {
		str, err := out.ReadString('\n')
		if err == io.EOF { // 读到文件末尾
			break
		} else {
			fmt.Printf("有数据了 %v \n", string(str))
		}
	}

	return self.dockerSdk.ContainerCreate(
		ctx,
		self.containerConfig,
		self.hostConfig,
		self.networkingConfig,
		self.platform,
		self.containerName,
	)
}
