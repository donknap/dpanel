package migrate

import (
	"fmt"
	"github.com/compose-spec/compose-go/v2/types"
	"github.com/docker/go-units"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/compose"
	"strconv"
	"strings"
)

type Upgrade20241014 struct{}

func (self Upgrade20241014) Version() string {
	return "1.2.0"
}

func (self Upgrade20241014) Upgrade() error {

	return nil
}

// todo 完全适配 compose spec 的参数
func (self Upgrade20241014) Covert(options []accessor.SiteEnvOption) types.Project {
	project := types.Project{
		Services: map[string]types.ServiceConfig{},
		Networks: make(types.Networks),
		Volumes:  make(types.Volumes),
	}
	extProject := compose.Ext{
		DisabledServices: make([]string, 0),
	}

	for _, siteOption := range options {
		service := types.ServiceConfig{
			Name:          siteOption.Name,
			Image:         siteOption.ImageName,
			ExternalLinks: make([]string, 0),
			Ports:         make([]types.ServicePortConfig, 0),
			Volumes:       make([]types.ServiceVolumeConfig, 0),
			Networks:      map[string]*types.ServiceNetworkConfig{},
			Privileged:    siteOption.Privileged,
			Restart:       siteOption.Restart,
			CPUS:          siteOption.Cpus,
			MemLimit:      types.UnitBytes(siteOption.Memory * 1024 * 1024),
			WorkingDir:    siteOption.WorkDir,
			User:          siteOption.User,
			//Command:       make(types.ShellCommand, 0),
			//Entrypoint:    make(types.ShellCommand, 0),
			LogDriver:  "",
			LogOpt:     make(map[string]string),
			DNS:        siteOption.Dns,
			Labels:     make(types.Labels),
			ExtraHosts: make(types.HostsList),
		}
		extService := compose.ExtService{
			External: compose.ExternalItem{
				VolumesFrom: make([]string, 0),
				Volumes:     make([]string, 0),
			},
			AutoRemove: siteOption.AutoRemove,
			Ports: compose.PortsItem{
				BindIPV6:   siteOption.BindIpV6,
				PublishAll: siteOption.PublishAllPorts,
			},
		}

		if !function.IsEmptyArray(siteOption.Environment) {
			envList := make([]string, 0)
			for _, item := range siteOption.Environment {
				envList = append(envList, fmt.Sprintf("%s=%s", item.Name, item.Value))
			}
			service.Environment = types.NewMappingWithEquals(envList)
		}

		// links 对应 compose 中的 external_links
		if !function.IsEmptyArray(siteOption.Links) {
			for _, item := range siteOption.Links {
				service.ExternalLinks = append(service.ExternalLinks, fmt.Sprintf("%s:%s", item.Name, item.Alise))
				if item.Volume {
					extService.External.VolumesFrom = append(extService.External.VolumesFrom, item.Name)
				}
			}
		}

		if !function.IsEmptyArray(siteOption.ReplaceDepend) {
			for _, item := range siteOption.ReplaceDepend {
				// 替换compose中服务时，部署时需要过滤掉
				if item.DependName != "" && item.ReplaceName != "" {
					service.ExternalLinks = append(service.ExternalLinks, fmt.Sprintf("%s:%s", item.ReplaceName, item.DependName))
					extProject.DisabledServices = append(extProject.DisabledServices, item.DependName)
				}
			}
		}

		for _, item := range siteOption.Ports {
			target, _ := strconv.Atoi(item.Dest)
			service.Ports = append(service.Ports, types.ServicePortConfig{
				HostIP:    item.HostIp,
				Target:    uint32(target),
				Published: item.Host,
			})
		}

		for _, item := range siteOption.Volumes {
			if !strings.Contains(item.Host, "/") {
				service.Volumes = append(service.Volumes, types.ServiceVolumeConfig{
					Type:     types.VolumeTypeVolume,
					Source:   item.Host,
					Target:   item.Dest,
					ReadOnly: item.Permission == "read",
				})
				project.Volumes[item.Host] = types.VolumeConfig{
					Name: item.Host,
				}
			} else {
				service.Volumes = append(service.Volumes, types.ServiceVolumeConfig{
					Type:     types.VolumeTypeBind,
					Source:   item.Host,
					Target:   item.Dest,
					ReadOnly: item.Permission == "read",
				})
			}
		}

		for _, item := range siteOption.VolumesDefault {
			volumePath := fmt.Sprintf("%s.%s", siteOption.Name, strings.Join(strings.Split(item.Dest, "/"), "-"))
			service.Volumes = append(service.Volumes, types.ServiceVolumeConfig{
				Source:   volumePath,
				Target:   item.Dest,
				ReadOnly: item.Permission == "read",
				Type:     types.VolumeTypeVolume,
			})
			project.Volumes[volumePath] = types.VolumeConfig{
				Name: volumePath,
			}
		}

		for _, item := range siteOption.Network {
			service.Networks[item.Name] = &types.ServiceNetworkConfig{
				Aliases:     item.Alise,
				Ipv4Address: item.IpV4,
				Ipv6Address: item.IpV6,
			}
			projectNetworkConfig := types.NetworkConfig{
				Name:     item.Name,
				External: true,
			}
			project.Networks[item.Name] = projectNetworkConfig
		}

		if siteOption.ShmSize != "" {
			size, _ := units.RAMInBytes(siteOption.ShmSize)
			service.ShmSize = types.UnitBytes(size)
		}

		if siteOption.Command != "" {
			service.Command = function.CommandSplit(siteOption.Command)
		}

		if siteOption.Entrypoint != "" {
			service.Entrypoint = function.CommandSplit(siteOption.Entrypoint)
		}

		if siteOption.UseHostNetwork {
			service.NetworkMode = "host"
		}

		if siteOption.Log.Driver != "" {
			loggingOpts := &types.LoggingConfig{
				Driver:  siteOption.Log.Driver,
				Options: make(types.Options),
			}
			if siteOption.Log.MaxSize != "" {
				loggingOpts.Options["max-size"] = siteOption.Log.MaxSize
			}
			if siteOption.Log.MaxFile != "" {
				loggingOpts.Options["max-file"] = siteOption.Log.MaxFile
			}
			service.Logging = loggingOpts
		}

		for _, item := range siteOption.Label {
			service.Labels[item.Name] = item.Value
		}

		hostList := make([]string, 0)
		for _, item := range siteOption.ExtraHosts {
			hostList = append(hostList, fmt.Sprintf("%s=%s", item.Name, item.Value))
		}
		if !function.IsEmptyArray(hostList) {
			hostLists, err := types.NewHostsList(hostList)
			if err != nil {
				service.ExtraHosts = make(types.HostsList)
			} else {
				service.ExtraHosts = hostLists
			}
		}

		service.Extensions = map[string]any{
			compose.ExtensionServiceName: extService,
		}

		if siteOption.IpV4.Address != "" || siteOption.IpV6.Address != "" {
			projectNetworkConfig := types.NetworkConfig{
				Name: siteOption.Name,
				Ipam: types.IPAMConfig{
					Driver: "default",
					Config: make([]*types.IPAMPool, 0),
				},
			}

			networkConfig := &types.ServiceNetworkConfig{}
			if siteOption.IpV4.Address != "" {
				networkConfig.Ipv4Address = siteOption.IpV4.Address
				projectNetworkConfig.Ipam.Config = append(projectNetworkConfig.Ipam.Config, &types.IPAMPool{
					Subnet:  siteOption.IpV4.Subnet,
					Gateway: siteOption.IpV4.Gateway,
				})
			}
			if siteOption.IpV6.Address != "" {
				projectNetworkConfig.EnableIPv6 = function.PtrBool(true)
				networkConfig.Ipv6Address = siteOption.IpV6.Address
				projectNetworkConfig.Ipam.Config = append(projectNetworkConfig.Ipam.Config, &types.IPAMPool{
					Subnet:  siteOption.IpV6.Subnet,
					Gateway: siteOption.IpV6.Gateway,
				})
			}
			service.Networks[siteOption.Name] = networkConfig
			project.Networks[siteOption.Name] = projectNetworkConfig
		}
		project.Services[siteOption.Name] = service
	}
	project.Extensions = map[string]any{
		compose.ExtensionName: extProject,
	}
	return project
}
