package stats

import (
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
)

// Optional time rates are seconds/second; unavailable samples stay nil.
func calculateCPUThrottled(previous, current *uint64, previousRead, read time.Time) *float64 {
	interval := read.Sub(previousRead)
	if previous == nil || current == nil || *current < *previous || interval <= 0 {
		return nil
	}
	value := float64(*current-*previous) / float64(interval)
	return &value
}

func calculateBlockIOWaiting(previous, current *[]container.BlkioStatEntry, previousRead, read time.Time) *float64 {
	interval := read.Sub(previousRead)
	if previous == nil || current == nil || interval <= 0 {
		return nil
	}
	previousValue, previousExists := calculateBlockIOWaitTime(*previous)
	currentValue, currentExists := calculateBlockIOWaitTime(*current)
	if !previousExists || !currentExists || currentValue < previousValue {
		return nil
	}
	value := float64(currentValue-previousValue) / float64(interval)
	return &value
}

func calculateBlockIOWaitTime(entries []container.BlkioStatEntry) (uint64, bool) {
	var readWrite, total uint64
	var hasReadWrite, hasTotal bool
	for _, entry := range entries {
		switch strings.ToLower(entry.Op) {
		case "read", "write":
			hasReadWrite = true
			readWrite += entry.Value
		case "total":
			hasTotal = true
			total += entry.Value
		}
	}
	if hasReadWrite {
		return readWrite, true
	}
	return total, hasTotal
}

func calculateCPUPercentUnix(previousCPU, previousSystem uint64, v *container.StatsResponse) float64 {
	var (
		cpuPercent = 0.0
		// calculate the change for the cpu usage of the container in between readings
		cpuDelta = float64(v.CPUStats.CPUUsage.TotalUsage) - float64(previousCPU)
		// calculate the change for the entire system between readings
		systemDelta = float64(v.CPUStats.SystemUsage) - float64(previousSystem)
		onlineCPUs  = float64(v.CPUStats.OnlineCPUs)
	)

	if onlineCPUs == 0.0 {
		onlineCPUs = float64(len(v.CPUStats.CPUUsage.PercpuUsage))
	}
	if systemDelta > 0.0 && cpuDelta > 0.0 {
		cpuPercent = (cpuDelta / systemDelta) * onlineCPUs * 100.0
	}
	return cpuPercent
}

func calculateCPUPercentWindows(v *container.StatsResponse) float64 {
	// Max number of 100ns intervals between the previous time read and now
	possIntervals := uint64(v.Read.Sub(v.PreRead).Nanoseconds()) // Start with number of ns intervals
	possIntervals /= 100                                         // Convert to number of 100ns intervals
	possIntervals *= uint64(v.NumProcs)                          // Multiple by the number of processors

	// Intervals used
	intervalsUsed := v.CPUStats.CPUUsage.TotalUsage - v.PreCPUStats.CPUUsage.TotalUsage

	// Percentage avoiding divide-by-zero
	if possIntervals > 0 {
		return float64(intervalsUsed) / float64(possIntervals) * 100.0
	}
	return 0.00
}

func calculateMemUsageUnixNoCache(mem container.MemoryStats) float64 {
	// cgroup v1
	if v, isCgroup1 := mem.Stats["total_inactive_file"]; isCgroup1 && v < mem.Usage {
		return float64(mem.Usage - v)
	}
	// cgroup v2
	if v := mem.Stats["inactive_file"]; v < mem.Usage {
		return float64(mem.Usage - v)
	}
	return float64(mem.Usage)
}

func calculateBlockIO(blkio container.BlkioStats) (uint64, uint64) {
	var blkRead, blkWrite uint64
	for _, bioEntry := range blkio.IoServiceBytesRecursive {
		if len(bioEntry.Op) == 0 {
			continue
		}
		switch bioEntry.Op[0] {
		case 'r', 'R':
			blkRead += bioEntry.Value
		case 'w', 'W':
			blkWrite += bioEntry.Value
		}
	}
	return blkRead, blkWrite
}

func calculateNetwork(network map[string]container.NetworkStats) (float64, float64) {
	var rx, tx float64

	for _, v := range network {
		rx += float64(v.RxBytes)
		tx += float64(v.TxBytes)
	}
	return rx, tx
}
