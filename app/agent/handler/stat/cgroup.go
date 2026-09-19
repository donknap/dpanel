//go:build linux

package stat

import (
	"bufio"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	agentTypes "github.com/donknap/dpanel/app/agent/types"
)

const hostCgroupPath = hostSysPath + "/fs/cgroup"

type containerReader struct {
	containers []containerTarget
	locations  map[string]containerCgroupLocation
	previous   map[string]agentTypes.ContainerStat
}

type containerCgroupLocation struct {
	unified string
	legacy  map[string]string
}

func newContainerReader(containers []containerTarget) *containerReader {
	return &containerReader{
		containers: containers,
		locations:  make(map[string]containerCgroupLocation),
		previous:   make(map[string]agentTypes.ContainerStat),
	}
}

func (reader *containerReader) Read(sampledAt time.Time, hostCPU cpuCounters, hostMemory uint64) []agentTypes.ContainerStat {
	if len(reader.containers) == 0 {
		return nil
	}
	result := make([]agentTypes.ContainerStat, 0, len(reader.containers))
	for _, target := range reader.containers {
		value, ok := reader.read(target, sampledAt, hostCPU, hostMemory)
		if !ok {
			delete(reader.locations, target.id)
			value, ok = reader.read(target, sampledAt, hostCPU, hostMemory)
		}
		if !ok {
			continue
		}
		if previous, exists := reader.previous[target.id]; exists {
			value.PreRead = previous.Read
			value.PreCPUStats = previous.CPUStats
		}
		reader.previous[target.id] = value
		result = append(result, value)
	}
	return result
}

func (reader *containerReader) read(target containerTarget, sampledAt time.Time, hostCPU cpuCounters, hostMemory uint64) (agentTypes.ContainerStat, bool) {
	location, exists := reader.locations[target.id]
	if !exists {
		var err error
		location, err = locateContainerCgroup(target)
		if err != nil {
			return agentTypes.ContainerStat{}, false
		}
		reader.locations[target.id] = location
	}
	return readContainerStat(target.id, location, sampledAt, hostCPU, hostMemory)
}

func locateContainerCgroup(target containerTarget) (containerCgroupLocation, error) {
	legacy, unified, err := parseCgroupFile(filepath.Join(hostProcPath, strconv.Itoa(target.pid), "cgroup"))
	if err != nil {
		return containerCgroupLocation{}, fmt.Errorf("parse host process cgroup: %w", err)
	}
	if unified != "" {
		name, ok := outerContainerCgroup(unified, target.id)
		if !ok {
			return containerCgroupLocation{}, errors.New("host process is not in the requested container cgroup")
		}
		return containerCgroupLocation{unified: filepath.Join(hostCgroupPath, name)}, nil
	}
	paths := locateLegacyControllerPaths(legacy, target.id)
	if len(paths) == 0 {
		return containerCgroupLocation{}, errors.New("host process is not in the requested container cgroup")
	}
	return containerCgroupLocation{legacy: paths}, nil
}

func parseCgroupFile(name string) (map[string]string, string, error) {
	file, err := os.Open(name)
	if err != nil {
		return nil, "", err
	}
	defer file.Close()

	legacy := make(map[string]string)
	var unified string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.SplitN(scanner.Text(), ":", 3)
		if len(fields) != 3 || fields[2] == "" || !filepath.IsAbs(fields[2]) || filepath.Clean(fields[2]) != fields[2] {
			return nil, "", fmt.Errorf("invalid cgroup entry %q", scanner.Text())
		}
		if fields[1] == "" {
			unified = fields[2]
			continue
		}
		for _, controller := range strings.Split(fields[1], ",") {
			if controller != "" {
				legacy[controller] = fields[2]
			}
		}
	}
	if err = scanner.Err(); err != nil {
		return nil, "", err
	}
	if unified == "" && len(legacy) == 0 {
		return nil, "", errors.New("cgroup data is empty")
	}
	return legacy, unified, nil
}

func readContainerStat(containerID string, location containerCgroupLocation, sampledAt time.Time, hostCPU cpuCounters, hostMemory uint64) (agentTypes.ContainerStat, bool) {
	result := agentTypes.ContainerStat{ID: containerID, OSType: "linux", Read: sampledAt}
	if location.unified != "" {
		readStatsV2(&result, location.unified, hostCPU, hostMemory)
	} else {
		readStatsV1(&result, location.legacy, hostCPU, hostMemory)
	}
	return result, result.CPUStats != nil || result.MemoryStats != nil
}

func readKeyValues(name string) (map[string]uint64, error) {
	file, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	result := make(map[string]uint64)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 2 {
			return nil, fmt.Errorf("invalid cgroup value in %q", name)
		}
		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse cgroup value in %q: %w", name, err)
		}
		result[fields[0]] = value
	}
	if err = scanner.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func readUintFile(name string) (uint64, error) {
	data, err := os.ReadFile(name)
	if err != nil {
		return 0, err
	}
	value := strings.TrimSpace(string(data))
	if value == "max" {
		return math.MaxUint64, nil
	}
	result, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse cgroup value in %q: %w", name, err)
	}
	return result, nil
}

func readUintList(name string) ([]uint64, error) {
	data, err := os.ReadFile(name)
	if err != nil {
		return nil, err
	}
	fields := strings.Fields(string(data))
	result := make([]uint64, len(fields))
	for index, field := range fields {
		result[index], err = strconv.ParseUint(field, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse cgroup value in %q: %w", name, err)
		}
	}
	return result, nil
}

func clampMemoryLimit(limit, hostMemory uint64) uint64 {
	if hostMemory > 0 && (limit == 0 || limit == math.MaxUint64 || limit > hostMemory) {
		return hostMemory
	}
	return limit
}
