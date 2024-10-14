package compose

import "github.com/compose-spec/compose-go/v2/types"

const ExtensionServiceName = "x-dpanel"
const ExtensionName = "x-dpanel"

const VolumeTypeDPanel = "dpanel"

type ExternalItem struct {
	VolumesFrom []string                               `yaml:"volumes_from,omitempty"`
	Volumes     []string                               `yaml:"volumes,omitempty"`
	Networks    map[string]*types.ServiceNetworkConfig `yaml:"networks,omitempty"`
}

type PortsItem struct {
	BindIPV6   bool `yaml:"bind_ipv6,omitempty"`
	PublishAll bool `yaml:"publish_all,omitempty"`
}
type ExtService struct {
	ImageTar   map[string]string `yaml:"image_tar,omitempty"`
	AutoRemove bool              `yaml:"auto_remove,omitempty"`
	External   ExternalItem      `yaml:"external,omitempty"` // 关联外部容器资源
	Ports      PortsItem         `yaml:"ports,omitempty"`
}

type Ext struct {
}

type Service struct {
	types.ServiceConfig
	XDPanel ExtService `yaml:"x-dpanel,omitempty"`
}
