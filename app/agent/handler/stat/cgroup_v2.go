//go:build linux

package stat

import "path/filepath"

func readStatsV2(result *containerStat, directory string, hostCPU cpuCounters, hostMemory uint64) {
	if items, err := readKeyValues(filepath.Join(directory, "cpu.stat")); err == nil {
		result.CPUStats = &dockerCPUStats{
			CPUUsage: dockerCPUUsage{
				TotalUsage: items["usage_usec"] * 1000, UsageInKernelmode: items["system_usec"] * 1000,
				UsageInUsermode: items["user_usec"] * 1000,
			},
			SystemUsage: hostCPU.dockerSystemUsage,
			OnlineCPUs:  uint32(hostCPU.cores),
			ThrottlingData: dockerThrottlingData{
				Periods: items["nr_periods"], ThrottledPeriods: items["nr_throttled"], ThrottledTime: items["throttled_usec"] * 1000,
			},
		}
	}
	usage, err := readUintFile(filepath.Join(directory, "memory.current"))
	if err != nil {
		return
	}
	value := &dockerMemoryStats{Usage: usage}
	if items, err := readKeyValues(filepath.Join(directory, "memory.stat")); err == nil {
		value.Stats = items
	}
	if limit, err := readUintFile(filepath.Join(directory, "memory.max")); err == nil {
		value.Limit = clampMemoryLimit(limit, hostMemory)
	}
	if maximum, err := readUintFile(filepath.Join(directory, "memory.peak")); err == nil {
		value.MaxUsage = maximum
	}
	if items, err := readKeyValues(filepath.Join(directory, "memory.events")); err == nil {
		value.Failcnt = items["oom"]
	}
	result.MemoryStats = value
}
