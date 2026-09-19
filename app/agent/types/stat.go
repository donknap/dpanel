package types

import "time"

const (
	PreviousStatSchemaVersion = 2
	StatSchemaVersion         = 3
)

type SystemStat struct {
	SchemaVersion  int             `json:"schemaVersion"`
	SampledAt      time.Time       `json:"sampledAt"`
	CPU            CPUStat         `json:"cpu"`
	Load           LoadStat        `json:"load"`
	Memory         MemoryStat      `json:"memory"`
	Pressure       *PressureStat   `json:"pressure,omitempty"`
	Disk           []DiskStat      `json:"disk,omitempty"`
	Network        *NetworkStat    `json:"network,omitempty"`
	ContainerStats []ContainerStat `json:"containerStats,omitempty"`
}

type CPUStat struct {
	Cores            int    `json:"cores"`
	Total            uint64 `json:"total"`
	User             uint64 `json:"user"`
	Nice             uint64 `json:"nice"`
	System           uint64 `json:"system"`
	Idle             uint64 `json:"idle"`
	IOWait           uint64 `json:"ioWait"`
	IRQ              uint64 `json:"irq"`
	SoftIRQ          uint64 `json:"softIRQ"`
	Steal            uint64 `json:"steal"`
	Guest            uint64 `json:"guest"`
	GuestNice        uint64 `json:"guestNice"`
	Interrupts       uint64 `json:"interrupts"`
	ContextSwitches  uint64 `json:"contextSwitches"`
	ProcessesCreated uint64 `json:"processesCreated"`
	ProcessesRunning uint64 `json:"processesRunning"`
	ProcessesBlocked uint64 `json:"processesBlocked"`
}

type LoadStat struct {
	Load1         float64 `json:"load1"`
	Load5         float64 `json:"load5"`
	Load15        float64 `json:"load15"`
	RunnableTasks uint64  `json:"runnableTasks"`
	TotalTasks    uint64  `json:"totalTasks"`
}

type MemoryStat struct {
	Total         uint64  `json:"total"`
	Available     uint64  `json:"available"`
	SwapTotal     uint64  `json:"swapTotal"`
	SwapAvailable uint64  `json:"swapAvailable"`
	Free          *uint64 `json:"free,omitempty"`
	Buffers       *uint64 `json:"buffers,omitempty"`
	Cached        *uint64 `json:"cached,omitempty"`
	SwapCached    *uint64 `json:"swapCached,omitempty"`
	Active        *uint64 `json:"active,omitempty"`
	Inactive      *uint64 `json:"inactive,omitempty"`
	ActiveAnon    *uint64 `json:"activeAnon,omitempty"`
	InactiveAnon  *uint64 `json:"inactiveAnon,omitempty"`
	ActiveFile    *uint64 `json:"activeFile,omitempty"`
	InactiveFile  *uint64 `json:"inactiveFile,omitempty"`
	AnonPages     *uint64 `json:"anonPages,omitempty"`
	Mapped        *uint64 `json:"mapped,omitempty"`
	Dirty         *uint64 `json:"dirty,omitempty"`
	Writeback     *uint64 `json:"writeback,omitempty"`
	Shmem         *uint64 `json:"shmem,omitempty"`
	Slab          *uint64 `json:"slab,omitempty"`
	SReclaimable  *uint64 `json:"sReclaimable,omitempty"`
	PageTables    *uint64 `json:"pageTables,omitempty"`
	KernelStack   *uint64 `json:"kernelStack,omitempty"`
}

type PressureStat struct {
	CPU    *PressureResource `json:"cpu,omitempty"`
	Memory *PressureResource `json:"memory,omitempty"`
	IO     *PressureResource `json:"io,omitempty"`
}

type PressureResource struct {
	Some PressureValue  `json:"some"`
	Full *PressureValue `json:"full,omitempty"`
}

type PressureValue struct {
	Avg10  float64 `json:"avg10"`
	Avg60  float64 `json:"avg60"`
	Avg300 float64 `json:"avg300"`
	Total  uint64  `json:"total"`
}

type DiskStat struct {
	Name                 string `json:"name"`
	ReadCount            uint64 `json:"readCount"`
	MergedReadCount      uint64 `json:"mergedReadCount"`
	WriteCount           uint64 `json:"writeCount"`
	MergedWriteCount     uint64 `json:"mergedWriteCount"`
	ReadBytes            uint64 `json:"readBytes"`
	WriteBytes           uint64 `json:"writeBytes"`
	ReadTimeMillis       uint64 `json:"readTimeMillis"`
	WriteTimeMillis      uint64 `json:"writeTimeMillis"`
	IOPSInProgress       uint64 `json:"iopsInProgress"`
	IOTimeMillis         uint64 `json:"ioTimeMillis"`
	WeightedIOTimeMillis uint64 `json:"weightedIOTimeMillis"`
}

type NetworkStat struct {
	ReceiveBytes  uint64 `json:"receiveBytes"`
	TransmitBytes uint64 `json:"transmitBytes"`
}
