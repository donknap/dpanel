//go:build linux

package stat

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	agentTypes "github.com/donknap/dpanel/app/agent/types"
)

const cgroupV1ClockTicks = 100

func locateLegacyControllerPaths(groups map[string]string, containerID string) map[string]string {
	result := make(map[string]string)
	for controller, group := range groups {
		if controller != "cpu" && controller != "cpuacct" && controller != "memory" {
			continue
		}
		outer, ok := outerContainerCgroup(group, containerID)
		if !ok {
			continue
		}
		if directory, exists := legacyControllerPath(controller, outer); exists {
			result[controller] = directory
		}
	}
	return result
}

func legacyControllerPath(controller, group string) (string, bool) {
	direct := filepath.Join(hostCgroupPath, controller, group)
	if info, err := os.Stat(direct); err == nil && info.IsDir() {
		return direct, true
	}
	entries, err := os.ReadDir(hostCgroupPath)
	if err != nil {
		return "", false
	}
	for _, entry := range entries {
		if !entry.IsDir() || !containsController(entry.Name(), controller) {
			continue
		}
		name := filepath.Join(hostCgroupPath, entry.Name(), group)
		if info, err := os.Stat(name); err == nil && info.IsDir() {
			return name, true
		}
	}
	return "", false
}

func containsController(value, target string) bool {
	for _, controller := range strings.Split(value, ",") {
		if controller == target {
			return true
		}
	}
	return false
}

func readStatsV1(result *agentTypes.ContainerStat, paths map[string]string, hostCPU cpuCounters, hostMemory uint64) {
	if directory := paths["cpuacct"]; directory != "" {
		if total, err := readUintFile(filepath.Join(directory, "cpuacct.usage")); err == nil {
			value := &agentTypes.CPUStats{
				CPUUsage:    agentTypes.CPUUsage{TotalUsage: total},
				SystemUsage: hostCPU.dockerSystemUsage,
				OnlineCPUs:  uint32(hostCPU.cores),
			}
			if items, err := readUintList(filepath.Join(directory, "cpuacct.usage_percpu")); err == nil {
				value.CPUUsage.PercpuUsage = items
			}
			if items, err := readKeyValues(filepath.Join(directory, "cpuacct.stat")); err == nil {
				value.CPUUsage.UsageInUsermode = ticksToNanoseconds(items["user"])
				value.CPUUsage.UsageInKernelmode = ticksToNanoseconds(items["system"])
			}
			result.CPUStats = value
		}
	}
	if result.CPUStats != nil {
		if directory := paths["cpu"]; directory != "" {
			if items, err := readKeyValues(filepath.Join(directory, "cpu.stat")); err == nil {
				result.CPUStats.ThrottlingData = agentTypes.ThrottlingData{
					Periods: items["nr_periods"], ThrottledPeriods: items["nr_throttled"], ThrottledTime: items["throttled_time"],
				}
			}
		}
	}
	if directory := paths["memory"]; directory != "" {
		usage, err := readUintFile(filepath.Join(directory, "memory.usage_in_bytes"))
		if err != nil {
			return
		}
		value := &agentTypes.MemoryStats{Usage: usage}
		if items, err := readKeyValues(filepath.Join(directory, "memory.stat")); err == nil {
			value.Stats = items
		}
		if limit, err := readUintFile(filepath.Join(directory, "memory.limit_in_bytes")); err == nil {
			value.Limit = clampMemoryLimit(limit, hostMemory)
		}
		if maximum, err := readUintFile(filepath.Join(directory, "memory.max_usage_in_bytes")); err == nil {
			value.MaxUsage = maximum
		}
		if failcnt, err := readUintFile(filepath.Join(directory, "memory.failcnt")); err == nil {
			value.Failcnt = failcnt
		}
		result.MemoryStats = value
	}
}

func ticksToNanoseconds(value uint64) uint64 {
	return value * uint64(time.Second) / cgroupV1ClockTicks
}
