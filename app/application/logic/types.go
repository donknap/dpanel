package logic

const (
	STATUS_STOP       = 0  // 未开始
	STATUS_PROCESSING = 10 // 进行中
	STATUS_ERROR      = 20 // 有错误
	STATUS_SUCCESS    = 30 // 部署成功
)

type MappingItem struct {
	Host string `json:"host"`
	Dest string `json:"dest"`
}

type LinkItem struct {
	Name  string `json:"name"`
	Alise string `json:"alise"`
}

type EnvItem struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type ContainerRunParams struct {
	Environment []EnvItem     `json:"environment"`
	Links       []LinkItem    `json:"links"`
	Ports       []MappingItem `json:"ports"`
	Volumes     []MappingItem `json:"volumes"`
}
