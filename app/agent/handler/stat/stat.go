package stat

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	gopsutilCommon "github.com/shirou/gopsutil/v4/common"
	"github.com/shirou/gopsutil/v4/disk"
)

const (
	hostProcPath = "/mnt_host_proc"
	hostSysPath  = "/mnt_host_sys"
)

type Handler struct{}

type systemStat struct {
	SampledAt      time.Time       `json:"sampledAt"`
	CPU            cpuStat         `json:"cpu"`
	Memory         memoryStat      `json:"memory"`
	Pressure       *pressureStat   `json:"pressure,omitempty"`
	Disk           *diskStat       `json:"disk,omitempty"`
	ContainerStats []containerStat `json:"containerStats,omitempty"`
}

type cpuStat struct {
	Cores        int     `json:"cores"`
	UsagePercent float64 `json:"usagePercent"`
	Load1        float64 `json:"load1"`
	Load5        float64 `json:"load5"`
	Load15       float64 `json:"load15"`
}

type memoryStat struct {
	Total         uint64 `json:"total"`
	Available     uint64 `json:"available"`
	SwapTotal     uint64 `json:"swapTotal"`
	SwapAvailable uint64 `json:"swapAvailable"`
}

type pressureStat struct {
	CPU    *pressureResource `json:"cpu,omitempty"`
	Memory *pressureResource `json:"memory,omitempty"`
	IO     *pressureResource `json:"io,omitempty"`
}

type pressureResource struct {
	Some pressureValue  `json:"some"`
	Full *pressureValue `json:"full,omitempty"`
}

type pressureValue struct {
	Avg10  float64 `json:"avg10"`
	Avg60  float64 `json:"avg60"`
	Avg300 float64 `json:"avg300"`
	Total  uint64  `json:"total"`
}

type diskStat struct {
	ReadBytesPerSecond       float64  `json:"readBytesPerSecond"`
	WriteBytesPerSecond      float64  `json:"writeBytesPerSecond"`
	ReadOperationsPerSecond  float64  `json:"readOperationsPerSecond"`
	WriteOperationsPerSecond float64  `json:"writeOperationsPerSecond"`
	BusyPercent              *float64 `json:"busyPercent,omitempty"`
}

type cpuCounters struct {
	total             uint64
	idle              uint64
	dockerSystemUsage uint64
	cores             int
}

type diskCounters struct {
	readOperations  uint64
	writeOperations uint64
	readBytes       uint64
	writeBytes      uint64
	busyMillis      uint64
	devices         uint64
}

func New() *Handler {
	return &Handler{}
}

func (*Handler) Handle(context.Context, []string) (any, error) {
	return nil, errors.New("stat is a streaming operation")
}

func (*Handler) HandleStream(ctx context.Context, args []string, write func(any) error) error {
	option, err := parseOptions(args)
	if err != nil {
		return err
	}
	containerReader := newContainerReader(option.containers)
	previousCPU, err := readCPU()
	if err != nil {
		return err
	}
	previousDisk, diskErr := readDisk(ctx)
	previousAt := time.Now()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case sampledAt := <-ticker.C:
			currentCPU, readErr := readCPU()
			if readErr != nil {
				return readErr
			}
			memory, readErr := readMemory()
			if readErr != nil {
				return readErr
			}
			load1, load5, load15, readErr := readLoad()
			if readErr != nil {
				return readErr
			}
			result := systemStat{
				SampledAt: sampledAt,
				CPU: cpuStat{
					Cores:        currentCPU.cores,
					UsagePercent: calculateCPU(previousCPU, currentCPU),
					Load1:        load1,
					Load5:        load5,
					Load15:       load15,
				},
				Memory:   memory,
				Pressure: readPressure(),
			}
			result.ContainerStats = containerReader.Read(sampledAt, currentCPU, memory.Total)
			currentDisk, readDiskErr := readDisk(ctx)
			if diskErr == nil && readDiskErr == nil {
				result.Disk = calculateDisk(previousDisk, currentDisk, sampledAt.Sub(previousAt))
			}
			previousCPU = currentCPU
			previousDisk = currentDisk
			diskErr = readDiskErr
			previousAt = sampledAt
			if err = write(result); err != nil {
				return fmt.Errorf("write stat sample: %w", err)
			}
		}
	}
}

func readCPU() (cpuCounters, error) {
	data, err := os.ReadFile(hostProcPath + "/stat")
	if err != nil {
		return cpuCounters{}, fmt.Errorf("read host cpu stat: %w", err)
	}
	result := cpuCounters{}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 0 {
			continue
		}
		if fields[0] == "cpu" {
			for index, field := range fields[1:] {
				value, parseErr := strconv.ParseUint(field, 10, 64)
				if parseErr != nil {
					return cpuCounters{}, fmt.Errorf("parse host cpu stat: %w", parseErr)
				}
				if index != 8 && index != 9 {
					result.total += value
				}
				if index <= 6 {
					result.dockerSystemUsage += value
				}
				if index == 3 || index == 4 {
					result.idle += value
				}
			}
		} else if strings.HasPrefix(fields[0], "cpu") && len(fields[0]) > 3 {
			if _, parseErr := strconv.Atoi(fields[0][3:]); parseErr == nil {
				result.cores++
			}
		}
	}
	if err = scanner.Err(); err != nil {
		return cpuCounters{}, fmt.Errorf("scan host cpu stat: %w", err)
	}
	if result.total == 0 || result.cores == 0 {
		return cpuCounters{}, errors.New("host cpu stat is empty")
	}
	result.dockerSystemUsage *= uint64(time.Second) / 100
	return result, nil
}

func calculateCPU(previous, current cpuCounters) float64 {
	if current.total <= previous.total || current.idle < previous.idle {
		return 0
	}
	totalDelta := current.total - previous.total
	idleDelta := current.idle - previous.idle
	if idleDelta >= totalDelta {
		return 0
	}
	return float64(totalDelta-idleDelta) / float64(totalDelta) * 100
}

func readLoad() (float64, float64, float64, error) {
	data, err := os.ReadFile(hostProcPath + "/loadavg")
	if err != nil {
		return 0, 0, 0, fmt.Errorf("read host load: %w", err)
	}
	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return 0, 0, 0, errors.New("host load data is incomplete")
	}
	values := make([]float64, 3)
	for index := range values {
		values[index], err = strconv.ParseFloat(fields[index], 64)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("parse host load: %w", err)
		}
	}
	return values[0], values[1], values[2], nil
}

func readMemory() (memoryStat, error) {
	data, err := os.ReadFile(hostProcPath + "/meminfo")
	if err != nil {
		return memoryStat{}, fmt.Errorf("read host memory: %w", err)
	}
	values := make(map[string]uint64)
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		value, parseErr := strconv.ParseUint(fields[1], 10, 64)
		if parseErr == nil {
			values[strings.TrimSuffix(fields[0], ":")] = value * 1024
		}
	}
	if err = scanner.Err(); err != nil {
		return memoryStat{}, fmt.Errorf("scan host memory: %w", err)
	}
	if values["MemTotal"] == 0 {
		return memoryStat{}, errors.New("host memory data is empty")
	}
	return memoryStat{
		Total:         values["MemTotal"],
		Available:     values["MemAvailable"],
		SwapTotal:     values["SwapTotal"],
		SwapAvailable: values["SwapFree"],
	}, nil
}

func readPressure() *pressureStat {
	result := &pressureStat{
		CPU:    readPressureResource("cpu"),
		Memory: readPressureResource("memory"),
		IO:     readPressureResource("io"),
	}
	if result.CPU == nil && result.Memory == nil && result.IO == nil {
		return nil
	}
	return result
}

func readPressureResource(name string) *pressureResource {
	data, err := os.ReadFile(hostProcPath + "/pressure/" + name)
	if err != nil {
		return nil
	}
	result := &pressureResource{}
	found := false
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		value := pressureValue{}
		valid := true
		for _, field := range fields[1:] {
			key, raw, ok := strings.Cut(field, "=")
			if !ok {
				valid = false
				break
			}
			switch key {
			case "avg10":
				value.Avg10, err = strconv.ParseFloat(raw, 64)
			case "avg60":
				value.Avg60, err = strconv.ParseFloat(raw, 64)
			case "avg300":
				value.Avg300, err = strconv.ParseFloat(raw, 64)
			case "total":
				value.Total, err = strconv.ParseUint(raw, 10, 64)
			}
			if err != nil {
				valid = false
				break
			}
		}
		if !valid {
			continue
		}
		switch fields[0] {
		case "some":
			result.Some = value
			found = true
		case "full":
			result.Full = &value
			found = true
		}
	}
	if !found {
		return nil
	}
	return result
}

func readDisk(ctx context.Context) (diskCounters, error) {
	ctx = context.WithValue(ctx, gopsutilCommon.EnvKey, gopsutilCommon.EnvMap{
		gopsutilCommon.HostProcEnvKey: hostProcPath,
		gopsutilCommon.HostSysEnvKey:  hostSysPath,
	})
	ioCounters, err := disk.IOCountersWithContext(ctx)
	if err != nil {
		return diskCounters{}, fmt.Errorf("read host disk stat: %w", err)
	}
	result := diskCounters{}
	for device, value := range ioCounters {
		devicePath := hostSysPath + "/block/" + device
		if _, err = os.Stat(devicePath + "/device"); err != nil {
			continue
		}
		slaves, readErr := os.ReadDir(devicePath + "/slaves")
		if readErr != nil || len(slaves) != 0 {
			continue
		}
		result.readOperations += value.ReadCount
		result.readBytes += value.ReadBytes
		result.writeOperations += value.WriteCount
		result.writeBytes += value.WriteBytes
		result.busyMillis += value.IoTime
		result.devices++
	}
	if result.devices == 0 {
		return diskCounters{}, errors.New("host disk stat is empty")
	}
	return result, nil
}

func calculateDisk(previous, current diskCounters, interval time.Duration) *diskStat {
	seconds := interval.Seconds()
	if seconds <= 0 || current.readOperations < previous.readOperations ||
		current.writeOperations < previous.writeOperations || current.readBytes < previous.readBytes ||
		current.writeBytes < previous.writeBytes || current.busyMillis < previous.busyMillis {
		return nil
	}
	result := &diskStat{
		ReadBytesPerSecond:       float64(current.readBytes-previous.readBytes) / seconds,
		WriteBytesPerSecond:      float64(current.writeBytes-previous.writeBytes) / seconds,
		ReadOperationsPerSecond:  float64(current.readOperations-previous.readOperations) / seconds,
		WriteOperationsPerSecond: float64(current.writeOperations-previous.writeOperations) / seconds,
	}
	if previous.devices > 0 && current.devices == previous.devices {
		busy := float64(current.busyMillis-previous.busyMillis) / (interval.Seconds() * 1000 * float64(current.devices)) * 100
		if busy > 100 {
			busy = 100
		}
		result.BusyPercent = &busy
	}
	return result
}
