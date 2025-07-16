package container

import (
	"fmt"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/swarm"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"strconv"
	"syscall"
	"time"
)

type Option func(self *Builder) error

func WithContainerInfo(config *container.Config, hostConfig *container.HostConfig) Option {
	return func(self *Builder) error {
		self.serviceSpec.TaskTemplate = swarm.TaskSpec{
			ContainerSpec: &swarm.ContainerSpec{
				Image:           config.Image,
				Labels:          config.Labels,
				Command:         config.Cmd,
				Args:            nil,
				Hostname:        config.Hostname,
				Env:             config.Env,
				Dir:             config.WorkingDir,
				User:            config.User,
				Privileges:      nil,
				Init:            function.Ptr(true),
				StopSignal:      syscall.SIGKILL.String(),
				TTY:             config.Tty,
				OpenStdin:       config.OpenStdin,
				ReadOnly:        hostConfig.ReadonlyRootfs,
				Mounts:          hostConfig.Mounts,
				StopGracePeriod: function.Ptr(time.Duration(*config.StopTimeout) * time.Second),
				Healthcheck:     config.Healthcheck,
				Hosts:           hostConfig.ExtraHosts,
				Isolation:       hostConfig.Isolation,
				Sysctls:         hostConfig.Sysctls,
				CapabilityAdd:   hostConfig.CapAdd,
				CapabilityDrop:  hostConfig.CapDrop,
				Ulimits:         hostConfig.Ulimits,
				OomScoreAdj:     int64(hostConfig.OomScoreAdj),
			},
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
			ports := make([]swarm.PortConfig, 0)
			for port, bindings := range hostConfig.PortBindings {
				for _, binding := range bindings {
					hostPort, _ := strconv.Atoi(binding.HostPort)
					ports = append(ports, swarm.PortConfig{
						Protocol:      swarm.PortConfigProtocol(port.Proto()),
						TargetPort:    uint32(port.Int()),
						PublishedPort: uint32(hostPort),
						PublishMode:   swarm.PortConfigPublishModeIngress,
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

func WithConstraintRole(values ...docker.ValueItem) Option {
	return func(self *Builder) error {
		self.serviceSpec.TaskTemplate.Placement.Constraints = function.PluckArrayWalk(values, func(item docker.ValueItem) (string, bool) {
			return fmt.Sprintf("%s==%s", item.Name, item.Value), true
		})
		return nil
	}
}
