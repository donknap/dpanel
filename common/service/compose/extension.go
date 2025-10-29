package compose

import "github.com/compose-spec/compose-go/v2/types"

const ExtensionServiceName = "x-dpanel-service"

type ExternalItem struct {
	VolumesFrom []string                               `yaml:"volumes_from,omitempty" json:"volumes_from"`
	Volumes     []string                               `yaml:"volumes,omitempty" json:"volumes"`
	Networks    map[string]*types.ServiceNetworkConfig `yaml:"networks,omitempty" json:"networks"`
}

type PortsItem struct {
	BindIPV6   bool `yaml:"bind_ipv6,omitempty" json:"bind_ipv6"`
	PublishAll bool `yaml:"publish_all,omitempty" json:"publish_all"`
}

type ExtService struct {
	ImageTar   map[string]string `yaml:"image_tar,omitempty" json:"image_tar"`
	AutoRemove bool              `yaml:"auto_remove,omitempty" json:"auto_remove"`
	External   ExternalItem      `yaml:"external,omitempty" json:"external"` // 关联外部容器资源
	Ports      PortsItem         `yaml:"ports,omitempty" json:"ports"`
}
