package compose

const ExtensionServiceName = "x-dpanel-service"
const ExtensionName = "x-dpanel"

type ExtService struct {
	ImageTar   map[string]string `yaml:"image_tar"`
	AutoRemove bool              `yaml:"auto_remove"`
	External   struct {
		VolumesFrom []string `yaml:"volumes_from"`
		Volumes     []string `yaml:"volumes"`
	} `yaml:"external"` // 关联外部容器存储
}

type Ext struct {
}
