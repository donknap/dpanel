package stat

import (
	"time"

	"github.com/docker/docker/api/types/container"
	agentTypes "github.com/donknap/dpanel/app/agent/types"
	"github.com/donknap/dpanel/common/service/docker/stats"
)

type Stat struct{}

// DockerStatFrame is one complete Docker container statistics sample.
type DockerStatFrame []*stats.Usage

// SystemStatFrame keeps the host sample and the Sysbox container corrections
// from the same agent sample together.
type SystemStatFrame struct {
	System     SystemStat
	Containers map[string]SystemContainerStat
}

type SystemContainerStat struct {
	Usage      *stats.Usage
	HasCPU     bool
	HasMemory  bool
	HasBlockIO bool
}

type SystemStat struct {
	SampledAt time.Time           `json:"sampledAt"`
	CPU       SystemCPUStat       `json:"cpu"`
	Memory    SystemMemoryStat    `json:"memory"`
	Pressure  *SystemPressureStat `json:"pressure,omitempty"`
	Disk      *SystemDiskStat     `json:"disk,omitempty"`
	Network   *SystemNetworkStat  `json:"network,omitempty"`
}

type SystemCPUStat struct {
	Cores         int      `json:"cores"`
	UsagePercent  float64  `json:"usagePercent"`
	IOWaitPercent *float64 `json:"ioWaitPercent,omitempty"`
	Load1         float64  `json:"load1"`
	Load5         float64  `json:"load5"`
	Load15        float64  `json:"load15"`
}

type SystemMemoryStat struct {
	Total         uint64 `json:"total"`
	Available     uint64 `json:"available"`
	SwapTotal     uint64 `json:"swapTotal"`
	SwapAvailable uint64 `json:"swapAvailable"`
}

type SystemPressureStat = agentTypes.PressureStat

type PressureStat = agentTypes.PressureResource

type PressureValue = agentTypes.PressureValue

type SystemDiskStat struct {
	ReadBytesPerSecond       float64  `json:"readBytesPerSecond"`
	WriteBytesPerSecond      float64  `json:"writeBytesPerSecond"`
	ReadOperationsPerSecond  float64  `json:"readOperationsPerSecond"`
	WriteOperationsPerSecond float64  `json:"writeOperationsPerSecond"`
	LatencyMillis            *float64 `json:"latencyMillis,omitempty"`
	BusyPercent              *float64 `json:"busyPercent,omitempty"`
}

type SystemNetworkStat struct {
	ReceiveBytesPerSecond  float64 `json:"receiveBytesPerSecond"`
	TransmitBytesPerSecond float64 `json:"transmitBytesPerSecond"`
}

type agentSystemBaseline struct {
	sampledAt time.Time
	cpu       agentTypes.CPUStat
	disks     map[string]agentTypes.DiskStat
	network   *agentTypes.NetworkStat
}

type agentSystemCollector struct {
	previous *agentSystemBaseline
}

// The schema-0 types stay local because they only describe the historical
// agent payload accepted by the main process for backward compatibility.
type legacyAgentSystemStat struct {
	SchemaVersion  int                        `json:"schemaVersion"`
	SampledAt      time.Time                  `json:"sampledAt"`
	CPU            legacyAgentCPUStat         `json:"cpu"`
	Memory         legacyAgentMemoryStat      `json:"memory"`
	Pressure       *agentTypes.PressureStat   `json:"pressure,omitempty"`
	Disk           *SystemDiskStat            `json:"disk,omitempty"`
	ContainerStats []legacyAgentContainerStat `json:"containerStats,omitempty"`
}

type legacyAgentCPUStat struct {
	Cores        int     `json:"cores"`
	UsagePercent float64 `json:"usagePercent"`
	Load1        float64 `json:"load1"`
	Load5        float64 `json:"load5"`
	Load15       float64 `json:"load15"`
}

type legacyAgentMemoryStat struct {
	Total         uint64 `json:"total"`
	Available     uint64 `json:"available"`
	SwapTotal     uint64 `json:"swapTotal"`
	SwapAvailable uint64 `json:"swapAvailable"`
}

type legacyAgentContainerStat struct {
	ID          string                 `json:"id"`
	OSType      string                 `json:"os_type"`
	Read        time.Time              `json:"read"`
	PreRead     time.Time              `json:"preread"`
	CPUStats    *container.CPUStats    `json:"cpu_stats,omitempty"`
	PreCPUStats *container.CPUStats    `json:"precpu_stats,omitempty"`
	MemoryStats *container.MemoryStats `json:"memory_stats,omitempty"`
	PidsStats   *container.PidsStats   `json:"pids_stats,omitempty"`
	BlkioStats  *container.BlkioStats  `json:"blkio_stats,omitempty"`
}

type containerStatSample struct {
	id      string
	osType  string
	read    time.Time
	preRead time.Time
	cpu     *container.CPUStats
	preCPU  *container.CPUStats
	memory  *container.MemoryStats
	pids    *container.PidsStats
	blkio   *container.BlkioStats
}

type systemStatSample struct {
	system     SystemStat
	containers map[string]SystemContainerStat
}
