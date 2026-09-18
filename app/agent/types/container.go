package types

import "time"

type ContainerStat struct {
	ID          string       `json:"id"`
	OSType      string       `json:"os_type"`
	Read        time.Time    `json:"read"`
	PreRead     time.Time    `json:"preread"`
	CPUStats    *CPUStats    `json:"cpu_stats,omitempty"`
	PreCPUStats *CPUStats    `json:"precpu_stats,omitempty"`
	MemoryStats *MemoryStats `json:"memory_stats,omitempty"`
	PidsStats   *PidsStats   `json:"pids_stats,omitempty"`
	BlkioStats  *BlkioStats  `json:"blkio_stats,omitempty"`
}

type CPUStats struct {
	CPUUsage       CPUUsage       `json:"cpu_usage"`
	SystemUsage    uint64         `json:"system_cpu_usage"`
	OnlineCPUs     uint32         `json:"online_cpus"`
	ThrottlingData ThrottlingData `json:"throttling_data"`
}

type CPUUsage struct {
	TotalUsage        uint64   `json:"total_usage"`
	PercpuUsage       []uint64 `json:"percpu_usage,omitempty"`
	UsageInKernelmode uint64   `json:"usage_in_kernelmode"`
	UsageInUsermode   uint64   `json:"usage_in_usermode"`
}

type ThrottlingData struct {
	Periods          uint64 `json:"periods"`
	ThrottledPeriods uint64 `json:"throttled_periods"`
	ThrottledTime    uint64 `json:"throttled_time"`
}

type MemoryStats struct {
	Usage    uint64            `json:"usage"`
	MaxUsage uint64            `json:"max_usage"`
	Stats    map[string]uint64 `json:"stats,omitempty"`
	Failcnt  uint64            `json:"failcnt"`
	Limit    uint64            `json:"limit"`
}

type BlkioStatEntry struct {
	Major uint64 `json:"major"`
	Minor uint64 `json:"minor"`
	Op    string `json:"op"`
	Value uint64 `json:"value"`
}

type BlkioStats struct {
	IoServiceBytesRecursive []BlkioStatEntry `json:"io_service_bytes_recursive"`
	IoServicedRecursive     []BlkioStatEntry `json:"io_serviced_recursive"`
	IoQueuedRecursive       []BlkioStatEntry `json:"io_queue_recursive"`
	IoServiceTimeRecursive  []BlkioStatEntry `json:"io_service_time_recursive"`
	IoWaitTimeRecursive     []BlkioStatEntry `json:"io_wait_time_recursive"`
	IoMergedRecursive       []BlkioStatEntry `json:"io_merged_recursive"`
	IoTimeRecursive         []BlkioStatEntry `json:"io_time_recursive"`
	SectorsRecursive        []BlkioStatEntry `json:"sectors_recursive"`
}

type PidsStats struct {
	Current uint64 `json:"current"`
	Limit   uint64 `json:"limit"`
}
