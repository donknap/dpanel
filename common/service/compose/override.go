package compose

import (
	"fmt"
	"github.com/compose-spec/compose-go/v2/types"
	"github.com/donknap/dpanel/common/accessor"
	"strconv"
)

func (self Wrapper) GetOverride(overrideList map[string]accessor.SiteEnvOption) types.Project {
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
			DependsOn:     oldService.DependsOn,
			Ports:         oldService.Ports,
			ExternalLinks: oldService.ExternalLinks,
		}
		for _, item := range override.Replace {
			if _, ok := self.Project.Services[name].DependsOn[item.Depend]; ok {
				newService.ExternalLinks = append(newService.ExternalLinks, fmt.Sprintf("%s:%s", item.Target, item.Depend))
				extProject.DisabledServices = append(extProject.DisabledServices, item.Depend)
				delete(newService.DependsOn, item.Depend)
			}
		}

		for _, item := range override.Ports {
			port := item.Parse()
			for newIndex, newItem := range newService.Ports {
				p, _ := strconv.Atoi(port.Dest)
				if newItem.Target == uint32(p) {
					if item.HostIp != "" {
						newService.Ports[newIndex].HostIP = item.HostIp
					}
					if item.Host == "" {
						newService.Ports[newIndex].Published = item.Host
					}
				}
			}
		}

		//for _, item := range override.Volumes {
		//
		//}
	}

	project.Extensions = map[string]any{
		ExtensionName: extProject,
	}
	return project
}
