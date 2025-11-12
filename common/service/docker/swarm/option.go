package swarm

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/compose-spec/compose-go/v2/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/swarm"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	containerBuilder "github.com/donknap/dpanel/common/service/docker/container"
	"github.com/donknap/dpanel/common/types/define"
)

type Option func(self *Builder) error

func WithContainerInfo(inspect container.InspectResponse) Option {
	config := inspect.Config
	hostConfig := inspect.HostConfig

	return func(self *Builder) error {
		self.serviceSpec.Name = strings.TrimPrefix(inspect.Name, "/")

		self.serviceSpec.TaskTemplate = swarm.TaskSpec{
			RestartPolicy: &swarm.RestartPolicy{},
			ContainerSpec: &swarm.ContainerSpec{
				Image:          config.Image,
				Labels:         config.Labels,
				Command:        config.Cmd,
				Args:           nil,
				Hostname:       config.Hostname,
				Env:            config.Env,
				Dir:            config.WorkingDir,
				User:           config.User,
				Privileges:     nil,
				StopSignal:     config.StopSignal,
				TTY:            config.Tty,
				OpenStdin:      config.OpenStdin,
				ReadOnly:       hostConfig.ReadonlyRootfs,
				Mounts:         hostConfig.Mounts,
				Healthcheck:    config.Healthcheck,
				Hosts:          hostConfig.ExtraHosts,
				Isolation:      hostConfig.Isolation,
				Sysctls:        hostConfig.Sysctls,
				CapabilityAdd:  hostConfig.CapAdd,
				CapabilityDrop: hostConfig.CapDrop,
				Ulimits:        hostConfig.Ulimits,
				OomScoreAdj:    int64(hostConfig.OomScoreAdj),
			},
		}
		if hostConfig.RestartPolicy.Name == types.RestartPolicyNo {
			self.serviceSpec.TaskTemplate.RestartPolicy.Condition = swarm.RestartPolicyConditionNone
		}
		if hostConfig.RestartPolicy.Name == types.RestartPolicyAlways || hostConfig.RestartPolicy.Name == types.RestartPolicyUnlessStopped {
			self.serviceSpec.TaskTemplate.RestartPolicy.Condition = swarm.RestartPolicyConditionAny
		}
		if hostConfig.RestartPolicy.Name == types.RestartPolicyOnFailure {
			self.serviceSpec.TaskTemplate.RestartPolicy.Condition = swarm.RestartPolicyConditionOnFailure
			self.serviceSpec.TaskTemplate.RestartPolicy.MaxAttempts = function.Ptr(uint64(hostConfig.RestartPolicy.MaximumRetryCount))
		}

		if config.StopTimeout != nil {
			self.serviceSpec.TaskTemplate.ContainerSpec.StopGracePeriod = function.Ptr(time.Duration(*config.StopTimeout) * time.Second)
		}

		if hostConfig.Init != nil {
			self.serviceSpec.TaskTemplate.ContainerSpec.Init = hostConfig.Init
		}

		if !function.IsEmptyArray(hostConfig.DNS) {
			self.serviceSpec.TaskTemplate.ContainerSpec.DNSConfig = &swarm.DNSConfig{
				Nameservers: hostConfig.DNS,
				Search:      hostConfig.DNSSearch,
				Options:     hostConfig.DNSOptions,
			}
		}

		if hostConfig.LogConfig.Type != "" {
			self.serviceSpec.TaskTemplate.LogDriver = &swarm.Driver{
				Name:    hostConfig.LogConfig.Type,
				Options: hostConfig.LogConfig.Config,
			}
		}

		if hostConfig.NanoCPUs != 0 {
			self.serviceSpec.TaskTemplate.Resources = &swarm.ResourceRequirements{
				Limits: &swarm.Limit{
					NanoCPUs:    hostConfig.NanoCPUs,
					MemoryBytes: hostConfig.Memory,
					Pids:        *hostConfig.PidsLimit,
				},
			}
		}

		if hostConfig.PortBindings != nil {
			var mode swarm.PortConfigPublishMode
			if hostConfig.NetworkMode == network.NetworkHost {
				mode = swarm.PortConfigPublishModeHost
			} else {
				mode = swarm.PortConfigPublishModeIngress
			}
			ports := make([]swarm.PortConfig, 0)
			for port, bindings := range hostConfig.PortBindings {
				for _, binding := range bindings {
					hostPort, _ := strconv.Atoi(binding.HostPort)
					ports = append(ports, swarm.PortConfig{
						Protocol:      swarm.PortConfigProtocol(port.Proto()),
						TargetPort:    uint32(port.Int()),
						PublishedPort: uint32(hostPort),
						PublishMode:   mode,
					})
				}
			}
			self.serviceSpec.EndpointSpec = &swarm.EndpointSpec{
				Mode:  swarm.ResolutionModeVIP,
				Ports: ports,
			}
		}

		return nil
	}
}

func WithContainerSecrets() Option {
	return func(self *Builder) error {
		self.serviceSpec.TaskTemplate.ContainerSpec.Secrets = nil
		return nil
	}
}

func WithContainerConfigs() Option {
	return func(self *Builder) error {
		self.serviceSpec.TaskTemplate.ContainerSpec.Configs = nil
		return nil
	}
}

func WithName(name string) Option {
	return func(self *Builder) error {
		self.serviceSpec.Annotations.Name = name
		return nil
	}
}

func WithLabel(values ...docker.ValueItem) Option {
	return func(self *Builder) error {
		if self.serviceSpec.Labels == nil {
			self.serviceSpec.Labels = make(map[string]string)
		}
		for _, item := range values {
			self.serviceSpec.Labels[item.Name] = item.Value
		}
		return nil
	}
}

func WithContainerSpec(option *accessor.SiteEnvOption) Option {
	options := make([]containerBuilder.Option, 0)
	options = append(options, []containerBuilder.Option{
		containerBuilder.WithHostname(option.Hostname),
		containerBuilder.WithImage(option.ImageName, false),
		containerBuilder.WithEnv(option.Environment...),
		containerBuilder.WithPort(option.Ports...),
		containerBuilder.WithVolume(option.Volumes...),
		containerBuilder.WithRestartPolicy(option.RestartPolicy),
		containerBuilder.WithCpus(option.Cpus),
		containerBuilder.WithMemory(option.Memory),

		containerBuilder.WithWorkDir(option.WorkDir),
		containerBuilder.WithUser(option.User),
		containerBuilder.WithCommandStr(option.Command),
		containerBuilder.WithEntrypointStr(option.Entrypoint),
		containerBuilder.WithLog(option.Log),
		containerBuilder.WithDns(option.Dns),
		containerBuilder.WithLabel(option.Label...),
		containerBuilder.WithExtraHosts(option.ExtraHosts...),
		containerBuilder.WithGpus(option.Gpus),
		containerBuilder.WithHealthcheck(option.Healthcheck),
		containerBuilder.WithCap(option.CapAdd...),
	}...)

	return func(self *Builder) error {
		hosts := make([]string, 0)
		for _, valueItem := range option.ExtraHosts {
			host := fmt.Sprintf("%s:%s", valueItem.Name, valueItem.Value)
			if !function.InArray(hosts, host) {
				hosts = append(hosts, host)
			}
		}
		self.serviceSpec.TaskTemplate.ContainerSpec = &swarm.ContainerSpec{
			Image: option.ImageName,
			Labels: function.PluckArrayMapWalk(option.Label, func(item docker.ValueItem) (string, string, bool) {
				return item.Name, item.Value, true
			}),
			Command:  function.SplitCommandArray(option.Command),
			Hostname: option.Hostname,
			Env: function.PluckArrayWalk(option.Environment, func(item docker.EnvItem) (string, bool) {
				return fmt.Sprintf("%s=%s", item.Name, item.Value), true
			}),
			Dir:       option.WorkDir,
			User:      option.User,
			Init:      function.Ptr(true),
			TTY:       true,
			OpenStdin: true,
			ReadOnly:  false,
			Hosts:     hosts,
			DNSConfig: &swarm.DNSConfig{
				Nameservers: option.Dns,
			},
			CapabilityAdd: option.CapAdd,
		}
		if option.Healthcheck != nil && option.Healthcheck.Cmd != "" {
			self.serviceSpec.TaskTemplate.ContainerSpec.Healthcheck = &container.HealthConfig{
				Timeout:  time.Duration(option.Healthcheck.Timeout) * time.Second,
				Retries:  option.Healthcheck.Retries,
				Interval: time.Duration(option.Healthcheck.Interval) * time.Second,
				Test: append([]string{
					option.Healthcheck.ShellType,
				}, function.SplitCommandArray(option.Healthcheck.Cmd)...),
			}
		}
		return nil
	}
}

func WithScheduling(value *docker.Scheduling) Option {
	return func(self *Builder) error {
		self.serviceSpec.UpdateConfig = &swarm.UpdateConfig{
			Parallelism:   uint64(value.Update.Parallelism),
			Delay:         time.Duration(value.Update.Delay) * time.Second,
			FailureAction: value.Update.FailureAction,
			Order:         value.Update.Order,
		}
		if value.Mode == define.SwarmServiceModeGlobal {
			self.serviceSpec.Mode = swarm.ServiceMode{
				Global: &swarm.GlobalService{},
			}
			return nil
		}
		if value.Mode == define.SwarmServiceModeReplicated {
			self.serviceSpec.Mode = swarm.ServiceMode{
				Replicated: &swarm.ReplicatedService{
					Replicas: function.Ptr(uint64(value.Replicas)),
				},
			}
			return nil
		}
		return errors.New("not support scheduling mode")
	}
}

func WithConstraint(value *docker.Constraint) Option {
	return func(self *Builder) error {
		if value == nil {
			return nil
		}
		if self.serviceSpec.TaskTemplate.Placement == nil {
			self.serviceSpec.TaskTemplate.Placement = &swarm.Placement{}
		}
		self.serviceSpec.TaskTemplate.Placement.Constraints = make([]string, 0)

		if value.Label != nil {
			for _, item := range value.Label {
				label := fmt.Sprintf("%s%s%s", item.Name, item.Operator, item.Value)
				if !function.InArray(self.serviceSpec.TaskTemplate.Placement.Constraints, label) {
					self.serviceSpec.TaskTemplate.Placement.Constraints = append(self.serviceSpec.TaskTemplate.Placement.Constraints, label)
				}
			}
		}

		if value.Role != "" {
			self.serviceSpec.TaskTemplate.Placement.Constraints = function.PluckArrayWalk(self.serviceSpec.TaskTemplate.Placement.Constraints, func(item string) (string, bool) {
				return item, !strings.HasPrefix(item, "node.role==") && !strings.HasPrefix(item, "node.role!=")
			})
			if value.Role != "all" && value.Role != "user" {
				self.serviceSpec.TaskTemplate.Placement.Constraints = append(self.serviceSpec.TaskTemplate.Placement.Constraints, fmt.Sprintf("node.role==%s", value.Role))
			}
		}

		if value.Node != "" {
			self.serviceSpec.TaskTemplate.Placement.Constraints = function.PluckArrayWalk(self.serviceSpec.TaskTemplate.Placement.Constraints, func(item string) (string, bool) {
				return item, !strings.HasPrefix(item, "node.id==")
			})
			self.serviceSpec.TaskTemplate.Placement.Constraints = append(self.serviceSpec.TaskTemplate.Placement.Constraints, fmt.Sprintf("node.id==%s", value.Node))
		}

		return nil
	}
}

func WithPlacement(values ...docker.ValueItem) Option {
	return func(self *Builder) error {
		if self.serviceSpec.TaskTemplate.Placement == nil {
			self.serviceSpec.TaskTemplate.Placement = &swarm.Placement{}
		}
		self.serviceSpec.TaskTemplate.Placement.Preferences = make([]swarm.PlacementPreference, 0)
		for _, item := range values {
			self.serviceSpec.TaskTemplate.Placement.Preferences = append(self.serviceSpec.TaskTemplate.Placement.Preferences, swarm.PlacementPreference{
				Spread: &swarm.SpreadOver{
					SpreadDescriptor: item.Value,
				},
			})
		}
		return nil
	}
}

func WithRestart(value *docker.RestartPolicy) Option {
	return func(self *Builder) error {
		self.serviceSpec.TaskTemplate.RestartPolicy = &swarm.RestartPolicy{
			Condition:   swarm.RestartPolicyCondition(value.Name),
			Delay:       function.Ptr(time.Duration(value.Delay) * time.Second),
			MaxAttempts: function.Ptr(uint64(value.MaxAttempt)),
			Window:      function.Ptr(time.Duration(value.Window) * time.Second),
		}
		return nil
	}
}

func WithRegistryAuth(code string) Option {
	return func(self *Builder) error {
		self.options.EncodedRegistryAuth = code
		return nil
	}
}

func WithPort(values ...docker.PortItem) Option {
	return func(self *Builder) error {
		ports := make([]swarm.PortConfig, 0)
		for _, portItem := range values {
			targetPort, _ := strconv.Atoi(portItem.Dest)
			publishPort, _ := strconv.Atoi(portItem.Host)
			portItem = portItem.Parse()
			ports = append(ports, swarm.PortConfig{
				Protocol:      swarm.PortConfigProtocol(portItem.Protocol),
				TargetPort:    uint32(targetPort),
				PublishedPort: uint32(publishPort),
				PublishMode:   swarm.PortConfigPublishMode(portItem.Mode),
			})
		}
		self.serviceSpec.EndpointSpec = &swarm.EndpointSpec{
			Mode:  swarm.ResolutionModeVIP,
			Ports: ports,
		}
		return nil
	}
}

func WithVolume(values ...docker.VolumeItem) Option {
	return func(self *Builder) error {
		self.serviceSpec.TaskTemplate.ContainerSpec.Mounts = function.PluckArrayWalk(values, func(item docker.VolumeItem) (mount.Mount, bool) {
			return mount.Mount{
				Type:     mount.Type(item.Type),
				Source:   item.Host,
				Target:   item.Dest,
				ReadOnly: item.Permission == "readonly",
			}, true
		})
		return nil
	}
}

func WithResourceLimit(cpu float32, memory, pid int) Option {
	return func(self *Builder) error {
		self.serviceSpec.TaskTemplate.Resources = &swarm.ResourceRequirements{
			Limits: &swarm.Limit{
				NanoCPUs:    int64(cpu * 1000000000),
				MemoryBytes: int64(memory) * 1024 * 1024,
				Pids:        int64(pid),
			},
		}
		return nil
	}
}

func WithServiceUpdate(service swarm.Service) Option {
	return func(self *Builder) error {
		self.Update = &service
		return nil
	}
}
