package stats

import (
	"math"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
)

type Usage struct {
	Cpu            float64                     `json:"cpu"`
	Memory         UsageIo                     `json:"memory"`
	PrevBlockIO    *UsageIo                    `json:"-"`
	BlockIO        UsageIo                     `json:"blockIO"`
	PrevNetworkIO  *UsageIo                    `json:"-"`
	NetworkIO      UsageIo                     `json:"networkIO"`
	Name           string                      `json:"name"`
	Container      string                      `json:"container"`
	CPUThrottled   *float64                    `json:"cpuThrottled,omitempty"`
	BlockIOWaiting *float64                    `json:"blockIOWaiting,omitempty"`
	Read           time.Time                   `json:"-"`
	ThrottledTime  *uint64                     `json:"-"`
	IOWaitTime     *[]container.BlkioStatEntry `json:"-"`
}

type UsageIo struct {
	In  float64 `json:"in"`
	Out float64 `json:"out"`
}

type Container struct {
	mutex sync.RWMutex
	err   error
	*Usage
}

func (self *Container) GetError() error {
	self.mutex.RLock()
	defer self.mutex.RUnlock()
	return self.err
}

func (self *Container) SetError(err error) {
	self.mutex.Lock()
	defer self.mutex.Unlock()
	self.err = err
}

func (self *Container) SetStatistics(v *container.StatsResponse, osType string, throttledTime *uint64, ioWaitTime *[]container.BlkioStatEntry) {
	self.mutex.Lock()
	defer self.mutex.Unlock()
	var (
		blkRead, blkWrite uint64
	)

	usage := &Usage{
		Cpu:           0,
		Memory:        UsageIo{},
		BlockIO:       UsageIo{},
		NetworkIO:     UsageIo{},
		Name:          v.Name,
		Container:     v.ID,
		Read:          v.Read,
		ThrottledTime: throttledTime,
		IOWaitTime:    ioWaitTime,
	}
	if osType != "windows" {
		if self.Usage != nil && self.Container == v.ID {
			usage.CPUThrottled = calculateCPUThrottled(self.ThrottledTime, throttledTime, self.Read, v.Read)
			usage.BlockIOWaiting = calculateBlockIOWaiting(self.IOWaitTime, ioWaitTime, self.Read, v.Read)
		}
		usage.Cpu = calculateCPUPercentUnix(v.PreCPUStats.CPUUsage.TotalUsage, v.PreCPUStats.SystemUsage, v)
		blkRead, blkWrite = calculateBlockIO(v.BlkioStats)

		// 写入值获取实时数据，减掉上一次的值
		// 上次数据没有时，直接返回 0
		if self.PrevBlockIO != nil {
			usage.BlockIO.In = math.Max(float64(blkWrite)-self.PrevBlockIO.In, 0)
			usage.BlockIO.Out = math.Max(float64(blkRead)-self.PrevBlockIO.Out, 0)
		}

		usage.PrevBlockIO = &UsageIo{
			In:  float64(blkWrite),
			Out: float64(blkRead),
		}

		usage.Memory.In = calculateMemUsageUnixNoCache(v.MemoryStats)
		usage.Memory.Out = float64(v.MemoryStats.Limit)
	} else {
		usage.Cpu = calculateCPUPercentWindows(v)
		usage.BlockIO.Out = float64(v.StorageStats.ReadSizeBytes)
		usage.BlockIO.In = float64(v.StorageStats.WriteSizeBytes)
		usage.Memory.In = float64(v.MemoryStats.PrivateWorkingSet)
	}

	netRead, netWrite := calculateNetwork(v.Networks)
	if self.PrevNetworkIO != nil {
		usage.NetworkIO.In = math.Max(netWrite-self.PrevNetworkIO.In, 0)
		usage.NetworkIO.Out = math.Max(netRead-self.PrevNetworkIO.Out, 0)
	}

	usage.PrevNetworkIO = &UsageIo{
		In:  netWrite,
		Out: netRead,
	}
	self.Usage = usage
}

func (self *Container) GetStatistics() *Usage {
	self.mutex.RLock()
	defer self.mutex.RUnlock()
	return self.Usage
}

func (self *Container) SetErrorAndReset() {
	self.mutex.Lock()
	defer self.mutex.Unlock()
	self.Usage = nil
}
