package compose

import (
	"bytes"
	"fmt"
	"github.com/compose-spec/compose-go/v2/types"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/function"
	"strconv"
	"strings"
)

func (self Wrapper) GetOverrideYaml(overrideList map[string]accessor.SiteEnvOption) ([]byte, error) {
	project := types.Project{
		Services: map[string]types.ServiceConfig{},
		Networks: make(types.Networks),
		Volumes:  make(types.Volumes),
	}
	extProject := Ext{
		DisabledServices: make([]string, 0),
	}

	for name, override := range overrideList {
		if _, ok := self.Project.Services[name]; !ok {
			continue
		}
		oldService := self.Project.Services[name]
		newService := types.ServiceConfig{
			Name:          name,
			ContainerName: override.ContainerName,
			DependsOn:     oldService.DependsOn,
			Ports:         oldService.Ports,
			ExternalLinks: oldService.ExternalLinks,
		}

		for _, item := range override.Replace {
			if _, ok := self.Project.Services[name].DependsOn[item.Depend]; ok && item.Target != "" {
				newService.ExternalLinks = append(newService.ExternalLinks, fmt.Sprintf("%s:%s", item.Target, item.Depend))
				extProject.DisabledServices = append(extProject.DisabledServices, item.Depend)
				delete(newService.DependsOn, item.Depend)
			}
		}

		for _, item := range override.Ports {
			port := item.Parse()
			p, _ := strconv.Atoi(port.Dest)
			exists, pos := function.FindArrayValueIndex(newService.Ports, "Target", uint32(p))
			if exists {
				if port.HostIp != "" {
					newService.Ports[pos[0]].HostIP = port.HostIp
				}
				if port.Host != "" {
					newService.Ports[pos[0]].Published = port.Host
				}
			} else {
				newService.Ports = append(newService.Ports, types.ServicePortConfig{
					HostIP:    port.HostIp,
					Published: port.Host,
					Target:    uint32(p),
					Protocol:  port.Protocol,
				})
			}
		}

		for _, item := range override.Volumes {
			exists, pos := function.FindArrayValueIndex(newService.Volumes, "Target", item.Dest)
			if exists {
				newService.Volumes[pos[0]].Source = item.Host
			} else {
				bindType := ""
				if strings.Contains(item.Host, "/") {
					bindType = types.VolumeTypeBind
				} else {
					bindType = types.VolumeTypeVolume
				}
				newService.Volumes = append(newService.Volumes, types.ServiceVolumeConfig{
					Type:     bindType,
					Source:   item.Host,
					Target:   item.Dest,
					ReadOnly: item.Permission == "read",
				})
				if bindType == types.VolumeTypeVolume {
					project.Volumes[item.Host] = types.VolumeConfig{
						Name: item.Host,
					}
				}
			}
		}

		newEnv := make(types.MappingWithEquals)
		for _, item := range override.Environment {
			newEnv[item.Name] = function.PtrString(item.Value)
		}
		newService.Environment = newEnv
		project.Services[name] = newService
	}

	project.Extensions = map[string]any{
		ExtensionName: extProject,
	}

	overrideYaml, err := project.MarshalYAML()
	if err != nil {
		return nil, err
	}
	// ports 配置要覆盖原始文件
	overrideYaml = bytes.Replace(overrideYaml, []byte("ports:"), []byte("ports: !override"), -1)
	overrideYaml = bytes.Replace(overrideYaml, []byte("depends_on:"), []byte("depends_on: !override"), -1)
	return overrideYaml, nil
}
