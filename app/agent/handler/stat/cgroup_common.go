package stat

import (
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const maxContainerIDOption = 4096

type statOptions struct {
	containers []containerTarget
}

type containerTarget struct {
	id  string
	pid int
}

type containerStat struct {
	ID          string             `json:"id"`
	OSType      string             `json:"os_type"`
	Read        time.Time          `json:"read"`
	PreRead     time.Time          `json:"preread"`
	CPUStats    *dockerCPUStats    `json:"cpu_stats,omitempty"`
	PreCPUStats *dockerCPUStats    `json:"precpu_stats,omitempty"`
	MemoryStats *dockerMemoryStats `json:"memory_stats,omitempty"`
	PidsStats   *dockerPidsStats   `json:"pids_stats,omitempty"`
	BlkioStats  *dockerBlkioStats  `json:"blkio_stats,omitempty"`
}

type dockerCPUStats struct {
	CPUUsage       dockerCPUUsage       `json:"cpu_usage"`
	SystemUsage    uint64               `json:"system_cpu_usage"`
	OnlineCPUs     uint32               `json:"online_cpus"`
	ThrottlingData dockerThrottlingData `json:"throttling_data"`
}

type dockerCPUUsage struct {
	TotalUsage        uint64   `json:"total_usage"`
	PercpuUsage       []uint64 `json:"percpu_usage,omitempty"`
	UsageInKernelmode uint64   `json:"usage_in_kernelmode"`
	UsageInUsermode   uint64   `json:"usage_in_usermode"`
}

type dockerThrottlingData struct {
	Periods          uint64 `json:"periods"`
	ThrottledPeriods uint64 `json:"throttled_periods"`
	ThrottledTime    uint64 `json:"throttled_time"`
}

type dockerMemoryStats struct {
	Usage    uint64            `json:"usage"`
	MaxUsage uint64            `json:"max_usage"`
	Stats    map[string]uint64 `json:"stats,omitempty"`
	Failcnt  uint64            `json:"failcnt"`
	Limit    uint64            `json:"limit"`
}

type dockerBlkioStatEntry struct {
	Major uint64 `json:"major"`
	Minor uint64 `json:"minor"`
	Op    string `json:"op"`
	Value uint64 `json:"value"`
}

type dockerBlkioStats struct {
	IoServiceBytesRecursive []dockerBlkioStatEntry `json:"io_service_bytes_recursive"`
	IoServicedRecursive     []dockerBlkioStatEntry `json:"io_serviced_recursive"`
	IoQueuedRecursive       []dockerBlkioStatEntry `json:"io_queue_recursive"`
	IoServiceTimeRecursive  []dockerBlkioStatEntry `json:"io_service_time_recursive"`
	IoWaitTimeRecursive     []dockerBlkioStatEntry `json:"io_wait_time_recursive"`
	IoMergedRecursive       []dockerBlkioStatEntry `json:"io_merged_recursive"`
	IoTimeRecursive         []dockerBlkioStatEntry `json:"io_time_recursive"`
	SectorsRecursive        []dockerBlkioStatEntry `json:"sectors_recursive"`
}

type dockerPidsStats struct {
	Current uint64 `json:"current"`
	Limit   uint64 `json:"limit"`
}

func parseOptions(args []string) (statOptions, error) {
	option := statOptions{containers: make([]containerTarget, 0)}
	seen := make(map[string]struct{})
	for len(args) > 0 {
		if args[0] != "--container-id" {
			return option, fmt.Errorf("unknown option: %s", args[0])
		}
		if len(args) < 2 {
			return option, errors.New("option --container-id requires a value")
		}
		target, err := parseContainerTarget(args[1])
		if err != nil {
			return option, err
		}
		args = args[2:]
		if _, exists := seen[target.id]; exists {
			return option, fmt.Errorf("duplicate container id %q", target.id)
		}
		if len(option.containers) >= maxContainerIDOption {
			return option, errors.New("too many --container-id options")
		}
		seen[target.id] = struct{}{}
		option.containers = append(option.containers, target)
	}
	return option, nil
}

func parseContainerTarget(value string) (containerTarget, error) {
	id, rawPID, found := strings.Cut(value, ":")
	if !found || !validContainerID(id) {
		return containerTarget{}, fmt.Errorf("invalid --container-id value %q, expected <64-char-id>:<host-pid>", value)
	}
	if rawPID == "" || rawPID[0] == '+' || len(rawPID) > 1 && rawPID[0] == '0' {
		return containerTarget{}, fmt.Errorf("invalid host pid in --container-id value %q", value)
	}
	pid, err := strconv.ParseInt(rawPID, 10, 32)
	if err != nil || pid <= 1 {
		return containerTarget{}, fmt.Errorf("invalid host pid in --container-id value %q", value)
	}
	return containerTarget{id: id, pid: int(pid)}, nil
}

func validContainerID(value string) bool {
	if len(value) != 64 || strings.ToLower(value) != value {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func outerContainerCgroup(path, containerID string) (string, bool) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return "", false
	}
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	for index, part := range parts {
		if part == containerID || strings.HasSuffix(part, "-"+containerID+".scope") {
			return "/" + strings.Join(parts[:index+1], "/"), true
		}
	}
	return "", false
}
