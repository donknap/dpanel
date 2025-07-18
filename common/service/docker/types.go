package docker

import (
	"archive/tar"
	"io"
	"os"
	"strings"
)

const (
	ImageBuildStatusStop    = 0  // 未开始
	ImageBuildStatusProcess = 10 // 进行中
	ImageBuildStatusError   = 20 // 有错误
	ImageBuildStatusSuccess = 30 // 部署成功
)

const (
	StepImagePull           = "imagePull"      // 拉取镜像中
	StepImageBuild          = "imageBuild"     // 开始构建镜像
	StepImageBuildUploadTar = "uploadTar"      // 上传构建 tar 包
	StepImageBuildRun       = "imageBuildRun"  // 开始执行dockerfile
	StepContainerBuild      = "containerBuild" // 创建容器
	StepContainerRun        = "containerRun"   // 运行容器
)

const (
	ContainerBackupTypeSnapshot = "snapshot"
)

const (
	RemoteTypeDocker = "docker"
	RemoteTypeSSH    = "ssh"
)

type PullMessage struct {
	Id             string `json:"id"`
	Status         string `json:"status"`
	Progress       string `json:"progress"`
	ProgressDetail struct {
		Current float64 `json:"current"`
		Total   float64 `json:"total"`
	} `json:"progressDetail"`
}

type BuildMessage struct {
	Stream      string `json:"stream"`
	ErrorDetail struct {
		Message string `json:"message"`
	} `json:"errorDetail"`
	PullMessage
}

type PullProgress struct {
	Downloading float64 `json:"downloading"`
	Extracting  float64 `json:"extracting"`
}

// 容器相关

type VolumeItem struct {
	Host       string `json:"host"`
	Dest       string `json:"dest"`
	Permission string `json:"permission"` // readonly or write
	InImage    bool   `json:"inImage,omitempty"`
	Type       string `json:"type,omitempty"`
}

type LinkItem struct {
	Name   string `json:"name"`
	Alise  string `json:"alise"`
	Volume bool   `json:"volume"`
}

type NetworkItem struct {
	Name       string   `json:"name"`
	Alise      []string `json:"alise"`
	IpV4       string   `json:"ipV4"`
	IpV6       string   `json:"ipV6"`
	MacAddress string   `json:"macAddress"`
	DnsName    []string `json:"dnsName"`
}

const (
	EnvValueRuleRequired = 1 << iota
	EnvValueRuleDisabled
	_
	_
	_
	_
	_
	_
	_
	_
	EnvValueTypeNumber
	EnvValueTypeText
	EnvValueTypeSelect
	EnvValueTypeSelectMultiple
)

type ValueRuleItem struct {
	Kind   int         `json:"kind,omitempty" yaml:"kind,omitempty"`
	Option []ValueItem `json:"option,omitempty" yaml:"option,omitempty"`
}

type EnvItem struct {
	Label  string            `json:"label,omitempty" yaml:"label,omitempty"`
	Labels map[string]string `json:"labels,omitempty"`
	Name   string            `json:"name"`
	Value  string            `json:"value"`
	Rule   *ValueRuleItem    `json:"rule,omitempty"`
}

type ValueItem struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type DeviceItem struct {
	Host string `json:"host"`
	Dest string `json:"dest"`
}

type PortItem struct {
	HostIp   string `json:"hostIp"`
	Host     string `json:"host"`
	Dest     string `json:"dest"`
	Protocol string `json:"protocol"`
	Mode     string `json:"mode"`
}

func (self *PortItem) Parse() PortItem {
	if hostIp, port, exists := strings.Cut(self.Host, ":"); exists {
		self.HostIp = hostIp
		self.Host = port
	}
	if port, protocol, exists := strings.Cut(self.Dest, "/"); exists {
		self.Dest = port
		self.Protocol = protocol
	}
	return *self
}

type LogDriverItem struct {
	Driver  string `json:"driver"`
	MaxFile string `json:"maxFile"`
	MaxSize string `json:"maxSize"`
}

type GpusItem struct {
	Enable       bool     `json:"enable"`
	Device       []string `json:"device"`
	Capabilities []string `json:"capabilities"`
}

type HookItem struct {
	ContainerStart  string `json:"containerStart"`
	ContainerCreate string `json:"containerCreate"`
}

type HealthcheckItem struct {
	ShellType string `json:"shellType"`
	Cmd       string `json:"cmd"`
	Interval  int    `json:"interval"`
	Timeout   int    `json:"timeout"`
	Retries   int    `json:"retries"`
}

type NetworkCreateItem struct {
	Address string `json:"address"`
	Subnet  string `json:"subnet"`
	Gateway string `json:"gateway"`
}

type ImagePlatform struct {
	Type string `json:"type"`
	Arch string `json:"arch"`
}

type ImportFile struct {
	targetRootPath string
	tarWrite       *tar.Writer
	reader         *os.File
	io.Closer
}

func (self ImportFile) Reader() io.Reader {
	_, _ = self.reader.Seek(0, io.SeekStart)
	return self.reader
}

func (self ImportFile) TarReader() *tar.Reader {
	_, _ = self.reader.Seek(0, io.SeekStart)
	return tar.NewReader(self.reader)
}

func (self ImportFile) Close() {
	_ = self.reader.Close()
	_ = os.Remove(self.reader.Name())
}

type ImportFileOption func(self *ImportFile) (err error)

type FileItemResult struct {
	ShowName string `json:"showName"` // 展示名称，包含名称 + link 名称
	Name     string `json:"name"`     // 完整的路径名称，不包含 linkname，eg: /dpanel/compose/compose1
	LinkName string `json:"linkName"` // 链接目录或是文件
	Size     string `json:"size"`
	Mode     string `json:"mode"`
	IsDir    bool   `json:"isDir"`
	ModTime  string `json:"modTime"`
	Change   int    `json:"change"`
	Group    string `json:"group"`
	Owner    string `json:"owner"`
}

type RestartPolicy struct {
	Name       string `json:"name"`
	MaxAttempt int    `json:"maxAttempt"`
	Delay      int    `json:"delay"`
	Window     int    `json:"window"`
}

type Constraint struct {
	Role  string   `json:"role"`
	Node  []string `json:"node"`
	Label []struct {
		Name     string `json:"name"`
		Value    string `json:"value"`
		Operator string `json:"operator"`
	} `json:"label"`
}

type Scheduling struct {
	Mode     string `json:"mode"`
	Replicas int    `json:"replicas,omitempty"`
	Update   struct {
		Delay         int    `json:"delay"`
		Parallelism   int    `json:"parallelism,omitempty"`
		FailureAction string `json:"failureAction"`
		Order         string `json:"order"`
	} `json:"update"`
}
