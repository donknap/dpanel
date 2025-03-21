package container

import (
	"errors"
	"fmt"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
	"github.com/docker/go-units"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"io"
	"os"
	"strings"
	"time"
)

type Option func(builder *Builder) error

func WithContainerInfo(containerInfo container.InspectResponse) Option {
	return func(self *Builder) error {
		self.containerConfig = containerInfo.Config
		self.hostConfig = containerInfo.HostConfig
		return nil
	}
}

func WithContainerName(name string) Option {
	return func(self *Builder) error {
		self.containerConfig.Hostname = fmt.Sprintf("%s.pod.dpanel.local", name)
		self.containerName = name
		//  防止退出
		self.containerConfig.AttachStdin = true
		self.containerConfig.AttachStdout = true
		self.containerConfig.AttachStderr = true
		self.containerConfig.Tty = true
		return nil
	}
}

func WithHostname(name string) Option {
	return func(self *Builder) error {
		self.containerConfig.Hostname = name
		return nil
	}
}

func WithDomainName(name string) Option {
	return func(self *Builder) error {
		self.containerConfig.Domainname = name
		return nil
	}
}

func WithEnv(item ...docker.EnvItem) Option {
	return func(self *Builder) error {
		self.containerConfig.Env = function.PluckArrayWalk(item, func(i docker.EnvItem) (string, bool) {
			if i.Name == "" {
				return "", false
			}
			return fmt.Sprintf("%s=%s", i.Name, i.Value), true
		})
		return nil
	}
}

func WithImage(imageName string, tryPullImage bool) Option {
	return func(self *Builder) error {
		if imageName == "" {
			return errors.New("image name is empty")
		}
		// 只尝试从 docker.io 拉取
		if tryPullImage {
			reader, err := docker.Sdk.Client.ImagePull(docker.Sdk.Ctx, imageName, image.PullOptions{})
			if err != nil {
				return err
			}
			_, err = io.Copy(os.Stdout, reader)
			if err != nil {
				return err
			}
		}
		self.containerConfig.Image = imageName
		return nil
	}
}

func WithRestartPolicy(restartPolicy string) Option {
	return func(self *Builder) error {
		if restartPolicy == "" {
			restartPolicy = "no"
		}
		self.hostConfig.RestartPolicy = container.RestartPolicy{}
		self.hostConfig.RestartPolicy.Name = docker.GetRestartPolicyByString(restartPolicy)
		return nil
	}
}

func WithPrivileged(b bool) Option {
	return func(self *Builder) error {
		self.hostConfig.Privileged = b
		return nil
	}
}

func WithVolume(item ...docker.VolumeItem) Option {
	return func(self *Builder) error {
		self.hostConfig.Binds = make([]string, 0)

		for _, volumeItem := range item {
			if volumeItem.Dest == "" || volumeItem.Host == "" {
				return errors.New("volume host path or dest path is empty")
			}
			permission := "rw"
			if volumeItem.Permission == "readonly" {
				permission = "ro"
			}
			bind := fmt.Sprintf("%s:%s:%s", volumeItem.Host, volumeItem.Dest, permission)
			if exists, index := function.IndexArrayWalk(self.hostConfig.Binds, func(i string) bool {
				return strings.Contains(i, ":"+volumeItem.Dest+":")
			}); exists {
				self.hostConfig.Binds[index] = bind
			} else {
				self.hostConfig.Binds = append(self.hostConfig.Binds, bind)
			}
		}
		return nil
	}
}

func WithVolumesFrom(item ...docker.LinkItem) Option {
	containerList := make([]string, 0)
	for _, linkItem := range item {
		if linkItem.Volume {
			containerList = append(containerList, linkItem.Name)
		}
	}
	return WithVolumesFromContainerName(containerList...)
}

func WithVolumesFromContainerName(item ...string) Option {
	return func(self *Builder) error {
		if self.hostConfig.VolumesFrom == nil {
			self.hostConfig.VolumesFrom = make([]string, 0)
		}
		for _, containerName := range item {
			if !function.InArray(self.hostConfig.VolumesFrom, containerName) {
				self.hostConfig.VolumesFrom = append(self.hostConfig.VolumesFrom, containerName)
			}
		}
		return nil
	}
}

func WithPort(item ...docker.PortItem) Option {
	return func(self *Builder) error {
		self.containerConfig.ExposedPorts = make(nat.PortSet)
		self.hostConfig.PortBindings = make(nat.PortMap)
		for _, portItem := range item {
			portItem = portItem.Parse()
			bind, err := nat.ParsePortSpec(fmt.Sprintf("%s:%s:%s/%s", portItem.HostIp, portItem.Host, portItem.Dest, portItem.Protocol))
			if err != nil {
				return err
			}
			for _, mapping := range bind {
				self.containerConfig.ExposedPorts[mapping.Port] = struct{}{}
				if self.hostConfig.PortBindings[mapping.Port] == nil {
					self.hostConfig.PortBindings[mapping.Port] = make([]nat.PortBinding, 0)
				}
				self.hostConfig.PortBindings[mapping.Port] = append(self.hostConfig.PortBindings[mapping.Port], mapping.Binding)
			}
		}
		return nil
	}
}

func WithPublishAllPorts(b bool) Option {
	return func(self *Builder) error {
		self.hostConfig.PublishAllPorts = b
		return nil
	}
}

func WithLink(item ...docker.LinkItem) Option {
	return func(self *Builder) error {
		for _, linkItem := range item {
			if linkItem.Alise == "" {
				continue
			}
			// 关联网络时，重新退出加入
			err := docker.Sdk.Client.NetworkDisconnect(docker.Sdk.Ctx, self.containerName, linkItem.Name, true)
			if err != nil {
				return err
			}
			return docker.Sdk.Client.NetworkConnect(docker.Sdk.Ctx, self.containerName, linkItem.Name, &network.EndpointSettings{
				Aliases: []string{
					linkItem.Alise,
				},
			})
		}
		return nil
	}
}

// WithNetwork 不在构建时加入网络，会导致 bridge 网络无法加入
func WithNetwork(item ...docker.NetworkItem) Option {
	return func(self *Builder) error {
		if self.networkingConfig == nil {
			self.networkingConfig = &network.NetworkingConfig{
				EndpointsConfig: make(map[string]*network.EndpointSettings),
			}
		}
		for _, networkRow := range item {
			if networkRow.Alise == nil {
				networkRow.Alise = make([]string, 0)
			}
			dpanelHostName := fmt.Sprintf("%s.pod.dpanel.local", self.containerName)
			if !function.InArray(networkRow.Alise, dpanelHostName) {
				networkRow.Alise = append(networkRow.Alise, dpanelHostName)
			}
			endpointSetting := &network.EndpointSettings{
				Aliases:    networkRow.Alise,
				IPAMConfig: &network.EndpointIPAMConfig{},
			}
			if networkRow.IpV4 != "" {
				endpointSetting.IPAMConfig.IPv4Address = networkRow.IpV4
			}
			if networkRow.IpV6 != "" {
				endpointSetting.IPAMConfig.IPv6Address = networkRow.IpV6
			}
			self.networkingConfig.EndpointsConfig[networkRow.Name] = endpointSetting
		}
		return nil
	}
}

func WithAutoRemove(b bool) Option {
	return func(self *Builder) error {
		self.hostConfig.AutoRemove = b
		return nil
	}
}

func WithCpus(count float32) Option {
	return func(self *Builder) error {
		if count == 0 {
			return nil
		}
		self.hostConfig.NanoCPUs = int64(count * 1000000000)
		return nil
	}
}

func WithMemory(size int) Option {
	return func(self *Builder) error {
		if size == 0 {
			return nil
		}
		self.hostConfig.Memory = int64(size) * 1024 * 1024
		return nil
	}
}

func WithShmSize(size string) Option {
	return func(self *Builder) error {
		d, _ := units.RAMInBytes(size)
		if d == 0 {
			return nil
		}
		self.hostConfig.ShmSize = d
		return nil
	}
}

func WithWorkDir(path string) Option {
	return func(self *Builder) error {
		if path == "" {
			return nil
		}
		self.containerConfig.WorkingDir = path
		return nil
	}
}

func WithUser(user string) Option {
	return func(self *Builder) error {
		if user == "" {
			return nil
		}
		self.containerConfig.User = user
		return nil
	}
}

func WithCommandStr(cmd string) Option {
	return WithCommand(docker.CommandSplit(cmd))
}

func WithCommand(cmd []string) Option {
	return func(self *Builder) error {
		if cmd == nil || len(cmd) == 0 {
			return nil
		}
		self.containerConfig.Cmd = cmd
		return nil
	}
}

func WithEntrypointStr(cmd string) Option {
	return WithEntrypoint(docker.CommandSplit(cmd))
}

func WithEntrypoint(cmd []string) Option {
	return func(self *Builder) error {
		if cmd == nil || len(cmd) == 0 {
			return nil
		}
		self.containerConfig.Entrypoint = cmd
		return nil
	}
}

func WithHostPid() Option {
	return WithPid("host")
}

func WithContainerPid(containerName string) Option {
	return WithPid(fmt.Sprintf("container:%s", containerName))
}

func WithPid(s string) Option {
	return func(self *Builder) error {
		self.hostConfig.PidMode = container.PidMode(s)
		return nil
	}
}

func WithHostNetwork() Option {
	return func(self *Builder) error {
		self.hostConfig.NetworkMode = network.NetworkHost
		return nil
	}
}

func WithContainerNetwork(containerName string) Option {
	return func(self *Builder) error {
		self.hostConfig.NetworkMode = container.NetworkMode(fmt.Sprintf("container:%s", containerName))
		return nil
	}
}

func WithLog(item *docker.LogDriverItem) Option {
	return func(self *Builder) error {
		if item == nil || item.Driver == "" {
			return nil
		}
		self.hostConfig.LogConfig = container.LogConfig{
			Type:   item.Driver,
			Config: make(map[string]string),
		}
		if item.MaxSize != "" {
			self.hostConfig.LogConfig.Config["max-size"] = item.MaxSize
		}
		if item.MaxFile != "" {
			self.hostConfig.LogConfig.Config["max-file"] = item.MaxFile
		}
		return nil
	}
}

func WithDns(ip []string) Option {
	return func(self *Builder) error {
		self.hostConfig.DNS = ip
		return nil
	}
}
func WithLabel(item ...docker.ValueItem) Option {
	return func(self *Builder) error {
		if self.containerConfig.Labels == nil {
			self.containerConfig.Labels = make(map[string]string)
		}
		for _, row := range item {
			self.containerConfig.Labels[row.Name] = row.Value
		}
		return nil
	}
}

func WithExtraHosts(item ...docker.ValueItem) Option {
	return func(self *Builder) error {
		self.hostConfig.ExtraHosts = make([]string, 0)

		for _, valueItem := range item {
			host := fmt.Sprintf("%s:%s", valueItem.Name, valueItem.Value)
			if !function.InArray(self.hostConfig.ExtraHosts, host) {
				self.hostConfig.ExtraHosts = append(self.hostConfig.ExtraHosts, host)
			}
		}
		return nil
	}
}

func WithDevice(item ...docker.DeviceItem) Option {
	return func(self *Builder) error {
		self.hostConfig.Devices = make([]container.DeviceMapping, 0)

		for _, deviceItem := range item {
			self.hostConfig.Devices = append(self.hostConfig.Devices, container.DeviceMapping{
				PathOnHost:        deviceItem.Host,
				PathInContainer:   deviceItem.Dest,
				CgroupPermissions: "rwm",
			})
		}
		return nil
	}
}

func WithGpus(item *docker.GpusItem) Option {
	return func(self *Builder) error {
		if item == nil || !item.Enable {
			return nil
		}
		self.hostConfig.DeviceRequests = make([]container.DeviceRequest, 0)

		if function.IsEmptyArray(item.Device) {
			item.Device = []string{
				"all",
			}
		}
		self.hostConfig.DeviceRequests = append(self.hostConfig.DeviceRequests, container.DeviceRequest{
			DeviceIDs: item.Device,
			Capabilities: [][]string{
				{
					"gpu",
				},
				item.Capabilities,
			},
			Driver: "nvidia",
		})
		return nil
	}
}

func WithHealthcheck(item *docker.HealthcheckItem) Option {
	return func(self *Builder) error {
		if item == nil || item.Cmd == "" {
			return nil
		}
		command := docker.CommandSplit(item.Cmd)
		self.containerConfig.Healthcheck = &container.HealthConfig{
			Timeout:  time.Duration(item.Timeout) * time.Second,
			Retries:  item.Retries,
			Interval: time.Duration(item.Interval) * time.Second,
			Test: append([]string{
				item.ShellType,
			}, command...),
		}
		return nil
	}
}

func WithCap(caps ...string) Option {
	return func(self *Builder) error {
		if function.IsEmptyArray(caps) {
			return nil
		}
		if function.InArray(caps, "ALL") {
			self.hostConfig.CapAdd = []string{
				"ALL",
			}
		} else {
			self.hostConfig.CapAdd = caps
		}
		return nil
	}
}
