package stat

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
	agentTypes "github.com/donknap/dpanel/app/agent/types"
	commonLogic "github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/docker/stats"
	"github.com/donknap/dpanel/common/service/plugin"
	"github.com/donknap/dpanel/common/types/define"
)

func (self Stat) ReadSystemStat(ctx context.Context, dockerSdk *docker.Client) <-chan SystemStatFrame {
	result := make(chan SystemStatFrame)
	go func() {
		defer close(result)

		err := streamSystem(ctx, dockerSdk, func(value systemStatSample) error {
			frame := SystemStatFrame{System: value.system, Containers: value.containers}
			select {
			case result <- frame:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
		if err != nil && ctx.Err() == nil && !errors.Is(err, context.Canceled) {
			slog.Warn("stream system stat", "dockerEnvName", dockerSdk.Name, "error", err)
		}
	}()
	return result
}

func streamSystem(ctx context.Context, dockerSdk *docker.Client, handle func(systemStatSample) error) error {
	command := []string{"/agent", "stat"}
	containerList, err := dockerSdk.Client.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		slog.Warn("list containers for agent stat", "dockerEnvName", dockerSdk.Name, "error", err)
	} else {
		for _, item := range containerList {
			containerInfo, inspectErr := dockerSdk.Client.ContainerInspect(ctx, item.ID)
			if inspectErr != nil {
				slog.Warn("inspect container for agent stat", "dockerEnvName", dockerSdk.Name,
					"container", item.ID, "error", inspectErr)
				continue
			}
			if containerInfo.HostConfig == nil || containerInfo.State == nil || containerInfo.State.Pid <= 0 ||
				!strings.Contains(strings.ToLower(containerInfo.HostConfig.Runtime), "sysbox") {
				continue
			}
			command = append(command, "--container-id", fmt.Sprintf("%s:%d", containerInfo.ID, containerInfo.State.Pid))
		}
	}
	_, response, err := dockerSdk.ContainerExec(ctx, plugin.MonitorName, container.ExecOptions{
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          command,
	})
	if err != nil {
		return err
	}
	defer response.Close()
	stopClose := context.AfterFunc(ctx, func() {
		_ = response.CloseWrite()
		response.Close()
	})
	defer stopClose()

	stdoutReader, stdoutWriter := io.Pipe()
	copyDone := make(chan error, 1)
	go func() {
		_, copyErr := stdcopy.StdCopy(stdoutWriter, io.Discard, response.Reader)
		_ = stdoutWriter.CloseWithError(copyErr)
		copyDone <- copyErr
	}()
	scanner := bufio.NewScanner(stdoutReader)
	scanner.Buffer(make([]byte, 64*1024), 64*1024*1024)
	systemCollector := &agentSystemCollector{}
	containerCollectors := make(map[string]*stats.Container)
	for scanner.Scan() {
		system, containers, ready, decodeErr := systemCollector.Decode(scanner.Bytes(), containerCollectors)
		if decodeErr != nil {
			return decodeErr
		}
		if !ready {
			continue
		}
		if err = handle(systemStatSample{system: system, containers: containers}); err != nil {
			return err
		}
	}
	if err = scanner.Err(); err != nil && ctx.Err() == nil {
		return fmt.Errorf("read agent stat: %w", err)
	}
	copyErr := <-copyDone
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return copyErr
}

func (collector *agentSystemCollector) Decode(
	data []byte, containerCollectors map[string]*stats.Container,
) (SystemStat, map[string]SystemContainerStat, bool, error) {
	var header struct {
		SchemaVersion int `json:"schemaVersion"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return SystemStat{}, nil, false, fmt.Errorf("decode agent stat schema: %w", err)
	}
	switch header.SchemaVersion {
	case 0:
		collector.previous = nil
		var value legacyAgentSystemStat
		if err := decodeAgentJSON(data, &value); err != nil {
			return SystemStat{}, nil, false, fmt.Errorf("decode legacy agent stat: %w", err)
		}
		return legacySystemStat(value), collectAgentContainerStats(legacyContainerSamples(value.ContainerStats), containerCollectors), true, nil
	case agentTypes.PreviousStatSchemaVersion, agentTypes.StatSchemaVersion:
		var value agentTypes.SystemStat
		if err := decodeAgentJSON(data, &value); err != nil {
			return SystemStat{}, nil, false, fmt.Errorf("decode agent stat schema %d: %w", header.SchemaVersion, err)
		}
		system, current, ready, err := deriveAgentSystemStat(collector.previous, value)
		if current != nil {
			collector.previous = current
		}
		if err != nil {
			return SystemStat{}, nil, false, err
		}
		containers := collectAgentContainerStats(agentContainerSamples(value.ContainerStats), containerCollectors)
		return system, containers, ready, nil
	default:
		return SystemStat{}, nil, false, fmt.Errorf("unsupported agent stat schema version %d", header.SchemaVersion)
	}
}

func decodeAgentJSON(data []byte, target any) error {
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return errors.New("value contains trailing data")
	}
	return nil
}

func legacySystemStat(value legacyAgentSystemStat) SystemStat {
	return SystemStat{
		SampledAt: value.SampledAt,
		CPU: SystemCPUStat{
			Cores: value.CPU.Cores, UsagePercent: value.CPU.UsagePercent,
			Load1: value.CPU.Load1, Load5: value.CPU.Load5, Load15: value.CPU.Load15,
		},
		Memory: SystemMemoryStat{
			Total: value.Memory.Total, Available: value.Memory.Available,
			SwapTotal: value.Memory.SwapTotal, SwapAvailable: value.Memory.SwapAvailable,
		},
		Pressure: value.Pressure,
		Disk:     value.Disk,
	}
}

func deriveAgentSystemStat(previous *agentSystemBaseline, value agentTypes.SystemStat) (
	SystemStat, *agentSystemBaseline, bool, error,
) {
	disks, err := indexAgentDisks(value.Disk)
	if err != nil {
		return SystemStat{}, nil, false, err
	}
	current := &agentSystemBaseline{
		sampledAt: value.SampledAt,
		cpu:       value.CPU,
		disks:     disks,
		network:   value.Network,
	}
	if value.CPU.Total == 0 {
		return SystemStat{}, current, false, fmt.Errorf("agent stat v%d CPU counters are incomplete", value.SchemaVersion)
	}
	if previous == nil {
		return SystemStat{}, current, false, nil
	}
	interval := value.SampledAt.Sub(previous.sampledAt)
	if interval <= 0 || agentCPUCounterRegressed(previous.cpu, value.CPU) {
		return SystemStat{}, current, false, nil
	}
	totalDelta := value.CPU.Total - previous.cpu.Total
	idleDelta := value.CPU.Idle - previous.cpu.Idle
	ioWaitDelta := value.CPU.IOWait - previous.cpu.IOWait
	if idleDelta > totalDelta || ioWaitDelta > totalDelta-idleDelta {
		return SystemStat{}, current, false, nil
	}
	usagePercent := float64(totalDelta-idleDelta-ioWaitDelta) / float64(totalDelta) * 100
	ioWaitPercent := float64(ioWaitDelta) / float64(totalDelta) * 100
	return SystemStat{
		SampledAt: value.SampledAt,
		CPU: SystemCPUStat{
			Cores: value.CPU.Cores, UsagePercent: usagePercent, IOWaitPercent: &ioWaitPercent,
			Load1: value.Load.Load1, Load5: value.Load.Load5, Load15: value.Load.Load15,
		},
		Memory: SystemMemoryStat{
			Total: value.Memory.Total, Available: value.Memory.Available,
			SwapTotal: value.Memory.SwapTotal, SwapAvailable: value.Memory.SwapAvailable,
		},
		Pressure: value.Pressure,
		Disk:     deriveAgentDiskStat(previous.disks, disks, interval),
		Network:  deriveAgentNetworkStat(previous.network, value.Network, interval),
	}, current, true, nil
}

func agentCPUCounterRegressed(previous, current agentTypes.CPUStat) bool {
	return current.Total <= previous.Total || current.User < previous.User || current.Nice < previous.Nice ||
		current.System < previous.System || current.Idle < previous.Idle || current.IOWait < previous.IOWait ||
		current.IRQ < previous.IRQ || current.SoftIRQ < previous.SoftIRQ || current.Steal < previous.Steal ||
		current.Guest < previous.Guest || current.GuestNice < previous.GuestNice
}

func indexAgentDisks(values []agentTypes.DiskStat) (map[string]agentTypes.DiskStat, error) {
	result := make(map[string]agentTypes.DiskStat, len(values))
	for _, value := range values {
		if value.Name == "" {
			return nil, errors.New("agent stat disk name is empty")
		}
		if _, exists := result[value.Name]; exists {
			return nil, fmt.Errorf("agent stat contains duplicate disk %q", value.Name)
		}
		result[value.Name] = value
	}
	return result, nil
}

func deriveAgentDiskStat(
	previous, current map[string]agentTypes.DiskStat, interval time.Duration,
) *SystemDiskStat {
	seconds := interval.Seconds()
	if seconds <= 0 {
		return nil
	}
	result := &SystemDiskStat{}
	validDevices := 0
	var busyMillis uint64
	var operationMillis uint64
	var operations uint64
	for name, value := range current {
		old, exists := previous[name]
		if !exists || agentDiskCounterRegressed(old, value) {
			continue
		}
		result.ReadBytesPerSecond += float64(value.ReadBytes-old.ReadBytes) / seconds
		result.WriteBytesPerSecond += float64(value.WriteBytes-old.WriteBytes) / seconds
		result.ReadOperationsPerSecond += float64(value.ReadCount-old.ReadCount) / seconds
		result.WriteOperationsPerSecond += float64(value.WriteCount-old.WriteCount) / seconds
		operationMillis += value.ReadTimeMillis - old.ReadTimeMillis + value.WriteTimeMillis - old.WriteTimeMillis
		operations += value.ReadCount - old.ReadCount + value.WriteCount - old.WriteCount
		busyMillis += value.IOTimeMillis - old.IOTimeMillis
		validDevices++
	}
	if validDevices == 0 {
		return nil
	}
	busyPercent := float64(busyMillis) / (float64(interval.Milliseconds()) * float64(validDevices)) * 100
	if busyPercent > 100 {
		busyPercent = 100
	}
	if operations > 0 {
		latencyMillis := float64(operationMillis) / float64(operations)
		result.LatencyMillis = &latencyMillis
	}
	result.BusyPercent = &busyPercent
	return result
}

func deriveAgentNetworkStat(
	previous, current *agentTypes.NetworkStat, interval time.Duration,
) *SystemNetworkStat {
	seconds := interval.Seconds()
	if previous == nil || current == nil || seconds <= 0 ||
		current.ReceiveBytes < previous.ReceiveBytes || current.TransmitBytes < previous.TransmitBytes {
		return nil
	}
	return &SystemNetworkStat{
		ReceiveBytesPerSecond:  float64(current.ReceiveBytes-previous.ReceiveBytes) / seconds,
		TransmitBytesPerSecond: float64(current.TransmitBytes-previous.TransmitBytes) / seconds,
	}
}

func agentDiskCounterRegressed(previous, current agentTypes.DiskStat) bool {
	return current.ReadCount < previous.ReadCount || current.MergedReadCount < previous.MergedReadCount ||
		current.WriteCount < previous.WriteCount || current.MergedWriteCount < previous.MergedWriteCount ||
		current.ReadBytes < previous.ReadBytes || current.WriteBytes < previous.WriteBytes ||
		current.ReadTimeMillis < previous.ReadTimeMillis || current.WriteTimeMillis < previous.WriteTimeMillis ||
		current.IOTimeMillis < previous.IOTimeMillis ||
		current.WeightedIOTimeMillis < previous.WeightedIOTimeMillis
}

func legacyContainerSamples(values []legacyAgentContainerStat) []containerStatSample {
	result := make([]containerStatSample, 0, len(values))
	for _, value := range values {
		result = append(result, containerStatSample{
			id: value.ID, osType: value.OSType, read: value.Read, preRead: value.PreRead,
			cpu: value.CPUStats, preCPU: value.PreCPUStats, memory: value.MemoryStats,
			pids: value.PidsStats, blkio: value.BlkioStats,
		})
	}
	return result
}

func agentContainerSamples(values []agentTypes.ContainerStat) []containerStatSample {
	result := make([]containerStatSample, 0, len(values))
	for _, value := range values {
		result = append(result, containerStatSample{
			id: value.ID, osType: value.OSType, read: value.Read, preRead: value.PreRead,
			cpu: dockerCPUStats(value.CPUStats), preCPU: dockerCPUStats(value.PreCPUStats),
			memory: dockerMemoryStats(value.MemoryStats), pids: dockerPidsStats(value.PidsStats),
			blkio: dockerBlkioStats(value.BlkioStats),
		})
	}
	return result
}

func dockerCPUStats(value *agentTypes.CPUStats) *container.CPUStats {
	if value == nil {
		return nil
	}
	return &container.CPUStats{
		CPUUsage: container.CPUUsage{
			TotalUsage: value.CPUUsage.TotalUsage, PercpuUsage: value.CPUUsage.PercpuUsage,
			UsageInKernelmode: value.CPUUsage.UsageInKernelmode, UsageInUsermode: value.CPUUsage.UsageInUsermode,
		},
		SystemUsage: value.SystemUsage,
		OnlineCPUs:  value.OnlineCPUs,
		ThrottlingData: container.ThrottlingData{
			Periods: value.ThrottlingData.Periods, ThrottledPeriods: value.ThrottlingData.ThrottledPeriods,
			ThrottledTime: value.ThrottlingData.ThrottledTime,
		},
	}
}

func dockerMemoryStats(value *agentTypes.MemoryStats) *container.MemoryStats {
	if value == nil {
		return nil
	}
	return &container.MemoryStats{
		Usage: value.Usage, MaxUsage: value.MaxUsage, Stats: value.Stats, Failcnt: value.Failcnt, Limit: value.Limit,
	}
}

func dockerPidsStats(value *agentTypes.PidsStats) *container.PidsStats {
	if value == nil {
		return nil
	}
	return &container.PidsStats{Current: value.Current, Limit: value.Limit}
}

func dockerBlkioStats(value *agentTypes.BlkioStats) *container.BlkioStats {
	if value == nil {
		return nil
	}
	return &container.BlkioStats{
		IoServiceBytesRecursive: dockerBlkioEntries(value.IoServiceBytesRecursive),
		IoServicedRecursive:     dockerBlkioEntries(value.IoServicedRecursive),
		IoQueuedRecursive:       dockerBlkioEntries(value.IoQueuedRecursive),
		IoServiceTimeRecursive:  dockerBlkioEntries(value.IoServiceTimeRecursive),
		IoWaitTimeRecursive:     dockerBlkioEntries(value.IoWaitTimeRecursive),
		IoMergedRecursive:       dockerBlkioEntries(value.IoMergedRecursive),
		IoTimeRecursive:         dockerBlkioEntries(value.IoTimeRecursive),
		SectorsRecursive:        dockerBlkioEntries(value.SectorsRecursive),
	}
}

func dockerBlkioEntries(values []agentTypes.BlkioStatEntry) []container.BlkioStatEntry {
	if values == nil {
		return nil
	}
	result := make([]container.BlkioStatEntry, len(values))
	for index, value := range values {
		result[index] = container.BlkioStatEntry{Major: value.Major, Minor: value.Minor, Op: value.Op, Value: value.Value}
	}
	return result
}

func collectAgentContainerStats(
	values []containerStatSample, collectors map[string]*stats.Container,
) map[string]SystemContainerStat {
	result := make(map[string]SystemContainerStat, len(values))
	for _, item := range values {
		cpuAvailable := item.cpu != nil && item.preCPU != nil
		memoryAvailable := item.memory != nil
		blockAvailable := item.blkio != nil
		if item.id == "" || (!cpuAvailable && !memoryAvailable && !blockAvailable) {
			continue
		}
		collector := collectors[item.id]
		if collector == nil {
			collector = &stats.Container{Usage: &stats.Usage{Container: item.id}}
			collectors[item.id] = collector
		}
		value := container.StatsResponse{ID: item.id, Read: item.read, PreRead: item.preRead}
		var throttledTime *uint64
		var ioWaitTime *[]container.BlkioStatEntry
		if cpuAvailable {
			value.CPUStats = *item.cpu
			value.PreCPUStats = *item.preCPU
			throttledTime = &value.CPUStats.ThrottlingData.ThrottledTime
		}
		if memoryAvailable {
			value.MemoryStats = *item.memory
		}
		if item.pids != nil {
			value.PidsStats = *item.pids
		}
		if blockAvailable {
			value.BlkioStats = *item.blkio
			ioWaitTime = &value.BlkioStats.IoWaitTimeRecursive
		}
		collector.SetStatistics(&value, item.osType, throttledTime, ioWaitTime)
		result[item.id] = SystemContainerStat{
			Usage:      collector.GetStatistics(),
			HasCPU:     cpuAvailable,
			HasMemory:  memoryAvailable,
			HasBlockIO: blockAvailable,
		}
	}
	return result
}

func (self Stat) HostDiskUsage(ctx context.Context, dockerSdk *docker.Client) (accessor.SystemDiskUsage, error) {
	output, err := dockerSdk.ContainerExecResult(ctx, plugin.MonitorName, container.ExecOptions{
		Cmd: []string{"/agent", "usage"},
	})
	if err != nil {
		return accessor.SystemDiskUsage{}, fmt.Errorf("execute agent usage: %w", err)
	}
	message := agentTypes.Message[*agentTypes.FilesystemUsage]{}
	if err = decodeAgentJSON([]byte(output), &message); err != nil {
		return accessor.SystemDiskUsage{}, fmt.Errorf("decode agent response: %w", err)
	}
	if message.Code != 200 || message.Error != "" || message.Data == nil {
		if message.Error == "" {
			message.Error = fmt.Sprintf("agent returned code %d", message.Code)
		}
		return accessor.SystemDiskUsage{}, errors.New(message.Error)
	}
	return accessor.SystemDiskUsage{
		Used: message.Data.Used, Available: message.Data.Available, Total: message.Data.Total,
		InodeUsed: message.Data.InodeUsed, InodeAvailable: message.Data.InodeAvailable,
		InodeTotal: message.Data.InodeTotal,
	}, nil
}

func (self Stat) ReconcileSystemStat(dockerSdk *docker.Client) error {
	if !dockerSdk.DockerEnv.EnableSystemStat {
		return nil
	}
	containerInfo, err := dockerSdk.Client.ContainerInspect(dockerSdk.Ctx, plugin.MonitorName)
	if err == nil && containerInfo.State != nil && containerInfo.State.Running &&
		containerInfo.Config != nil && containerInfo.Config.Labels[define.DPanelLabelContainerName] == plugin.MonitorName {
		return nil
	}
	if err != nil && !errdefs.IsNotFound(err) {
		return err
	}
	dockerSdk.DockerEnv.EnableSystemStat = false
	commonLogic.Env{}.UpdateEnv(dockerSdk.DockerEnv)
	return nil
}
