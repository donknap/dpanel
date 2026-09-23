package stat

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/donknap/dpanel/app/agent/internal/statparser"
	agentTypes "github.com/donknap/dpanel/app/agent/types"
	gopsutilCommon "github.com/shirou/gopsutil/v4/common"
	"github.com/shirou/gopsutil/v4/disk"
)

const (
	hostProcPath     = "/mnt_host_proc"
	hostSysPath      = "/mnt_host_sys"
	heartbeatTimeout = 6 * time.Second
)

type Handler struct{}

type cpuCounters struct {
	stat              agentTypes.CPUStat
	dockerSystemUsage uint64
	cores             int
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
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	watchHeartbeat(ctx, cancel, os.Stdin)
	containerReader := newContainerReader(option.containers)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	sampledAt := time.Now()

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		currentCPU, readErr := readCPU()
		if readErr != nil {
			return readErr
		}
		memory, readErr := readMemory()
		if readErr != nil {
			return readErr
		}
		load, readErr := readLoad()
		if readErr != nil {
			return readErr
		}
		result := agentTypes.SystemStat{
			SchemaVersion:  agentTypes.StatSchemaVersion,
			SampledAt:      sampledAt,
			CPU:            currentCPU.stat,
			Load:           load,
			Memory:         memory,
			Pressure:       readPressure(),
			ContainerStats: containerReader.Read(sampledAt, currentCPU, memory.Total),
		}
		result.Network, _ = readNetwork()
		if result.Disk, readErr = readDisk(ctx); readErr != nil {
			result.Disk = nil
		}
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		if err = write(result); err != nil {
			return fmt.Errorf("write stat sample: %w", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case sampledAt = <-ticker.C:
		}
	}
}

func watchHeartbeat(ctx context.Context, cancel context.CancelFunc, input io.Reader) {
	commands := make(chan string)
	inputDone := make(chan struct{})
	go func() {
		defer close(inputDone)
		scanner := bufio.NewScanner(input)
		for scanner.Scan() {
			select {
			case commands <- strings.TrimSpace(scanner.Text()):
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() {
		timer := time.NewTimer(heartbeatTimeout)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-inputDone:
				cancel()
				return
			case <-timer.C:
				cancel()
				return
			case command := <-commands:
				switch command {
				case "ping":
					if !timer.Stop() {
						select {
						case <-timer.C:
						default:
						}
					}
					timer.Reset(heartbeatTimeout)
				case "close":
					cancel()
					return
				}
			}
		}
	}()
}

func readNetwork() (*agentTypes.NetworkStat, error) {
	interfaces, err := os.ReadDir(hostSysPath + "/class/net")
	if err != nil {
		return nil, fmt.Errorf("read host network interfaces: %w", err)
	}
	physicalInterfaces := make(map[string]struct{})
	for _, item := range interfaces {
		if item.Name() == "lo" {
			continue
		}
		if _, err = os.Stat(hostSysPath + "/class/net/" + item.Name() + "/device"); err == nil {
			physicalInterfaces[item.Name()] = struct{}{}
		}
	}
	data, err := os.ReadFile(hostProcPath + "/1/net/dev")
	if err != nil {
		return nil, fmt.Errorf("read host network stat: %w", err)
	}
	return statparser.ParseNetwork(data, physicalInterfaces)
}

func readCPU() (cpuCounters, error) {
	data, err := os.ReadFile(hostProcPath + "/stat")
	if err != nil {
		return cpuCounters{}, fmt.Errorf("read host cpu stat: %w", err)
	}
	parsed, err := statparser.ParseCPU(data)
	if err != nil {
		return cpuCounters{}, err
	}
	return cpuCounters{stat: parsed.Stat, dockerSystemUsage: parsed.DockerSystemUsage, cores: parsed.Stat.Cores}, nil
}

func readLoad() (agentTypes.LoadStat, error) {
	data, err := os.ReadFile(hostProcPath + "/loadavg")
	if err != nil {
		return agentTypes.LoadStat{}, fmt.Errorf("read host load: %w", err)
	}
	return statparser.ParseLoad(data)
}

func readMemory() (agentTypes.MemoryStat, error) {
	data, err := os.ReadFile(hostProcPath + "/meminfo")
	if err != nil {
		return agentTypes.MemoryStat{}, fmt.Errorf("read host memory: %w", err)
	}
	return statparser.ParseMemory(data)
}

func readPressure() *agentTypes.PressureStat {
	result := &agentTypes.PressureStat{
		CPU:    readPressureResource("cpu"),
		Memory: readPressureResource("memory"),
		IO:     readPressureResource("io"),
	}
	if result.CPU == nil && result.Memory == nil && result.IO == nil {
		return nil
	}
	return result
}

func readPressureResource(name string) *agentTypes.PressureResource {
	data, err := os.ReadFile(hostProcPath + "/pressure/" + name)
	if err != nil {
		return nil
	}
	result, _ := statparser.ParsePressure(data)
	return result
}

func readDisk(ctx context.Context) ([]agentTypes.DiskStat, error) {
	ctx = context.WithValue(ctx, gopsutilCommon.EnvKey, gopsutilCommon.EnvMap{
		gopsutilCommon.HostProcEnvKey: hostProcPath,
		gopsutilCommon.HostSysEnvKey:  hostSysPath,
	})
	ioCounters, err := disk.IOCountersWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("read host disk stat: %w", err)
	}
	result := make([]agentTypes.DiskStat, 0, len(ioCounters))
	for device, value := range ioCounters {
		devicePath := hostSysPath + "/block/" + device
		if _, err = os.Stat(devicePath + "/device"); err != nil {
			continue
		}
		if deviceType, readErr := os.ReadFile(devicePath + "/device/type"); readErr == nil &&
			strings.TrimSpace(string(deviceType)) != "0" {
			continue
		}
		slaves, readErr := os.ReadDir(devicePath + "/slaves")
		if readErr != nil || len(slaves) != 0 {
			continue
		}
		result = append(result, agentTypes.DiskStat{
			Name:                 device,
			ReadCount:            value.ReadCount,
			MergedReadCount:      value.MergedReadCount,
			WriteCount:           value.WriteCount,
			MergedWriteCount:     value.MergedWriteCount,
			ReadBytes:            value.ReadBytes,
			WriteBytes:           value.WriteBytes,
			ReadTimeMillis:       value.ReadTime,
			WriteTimeMillis:      value.WriteTime,
			IOPSInProgress:       value.IopsInProgress,
			IOTimeMillis:         value.IoTime,
			WeightedIOTimeMillis: value.WeightedIO,
		})
	}
	if len(result) == 0 {
		return nil, errors.New("host disk stat is empty")
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}
