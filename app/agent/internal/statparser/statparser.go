package statparser

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	agentTypes "github.com/donknap/dpanel/app/agent/types"
)

type CPUResult struct {
	Stat              agentTypes.CPUStat
	DockerSystemUsage uint64
}

func ParseCPU(data []byte) (CPUResult, error) {
	result := CPUResult{}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		switch fields[0] {
		case "cpu":
			for index, field := range fields[1:] {
				value, err := strconv.ParseUint(field, 10, 64)
				if err != nil {
					return CPUResult{}, fmt.Errorf("parse host cpu stat: %w", err)
				}
				if index <= 7 {
					result.Stat.Total += value
				}
				if index <= 6 {
					result.DockerSystemUsage += value
				}
				switch index {
				case 0:
					result.Stat.User = value
				case 1:
					result.Stat.Nice = value
				case 2:
					result.Stat.System = value
				case 3:
					result.Stat.Idle = value
				case 4:
					result.Stat.IOWait = value
				case 5:
					result.Stat.IRQ = value
				case 6:
					result.Stat.SoftIRQ = value
				case 7:
					result.Stat.Steal = value
				case 8:
					result.Stat.Guest = value
				case 9:
					result.Stat.GuestNice = value
				}
			}
		case "intr", "ctxt", "processes", "procs_running", "procs_blocked":
			if len(fields) < 2 {
				return CPUResult{}, fmt.Errorf("host cpu stat field %s is incomplete", fields[0])
			}
			value, err := strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				return CPUResult{}, fmt.Errorf("parse host cpu stat field %s: %w", fields[0], err)
			}
			switch fields[0] {
			case "intr":
				result.Stat.Interrupts = value
			case "ctxt":
				result.Stat.ContextSwitches = value
			case "processes":
				result.Stat.ProcessesCreated = value
			case "procs_running":
				result.Stat.ProcessesRunning = value
			case "procs_blocked":
				result.Stat.ProcessesBlocked = value
			}
		default:
			if !strings.HasPrefix(fields[0], "cpu") || len(fields[0]) <= 3 {
				continue
			}
			if _, err := strconv.Atoi(fields[0][3:]); err == nil {
				result.Stat.Cores++
			}
		}
	}
	if result.Stat.Total == 0 || result.Stat.Cores == 0 {
		return CPUResult{}, errors.New("host cpu stat is empty")
	}
	result.DockerSystemUsage *= uint64(time.Second) / 100
	return result, nil
}

func ParseLoad(data []byte) (agentTypes.LoadStat, error) {
	fields := strings.Fields(string(data))
	if len(fields) < 4 {
		return agentTypes.LoadStat{}, errors.New("host load data is incomplete")
	}
	values := make([]float64, 3)
	for index := range values {
		value, err := strconv.ParseFloat(fields[index], 64)
		if err != nil {
			return agentTypes.LoadStat{}, fmt.Errorf("parse host load: %w", err)
		}
		values[index] = value
	}
	runnable, total, found := strings.Cut(fields[3], "/")
	if !found {
		return agentTypes.LoadStat{}, errors.New("host load task data is incomplete")
	}
	result := agentTypes.LoadStat{Load1: values[0], Load5: values[1], Load15: values[2]}
	var err error
	result.RunnableTasks, err = strconv.ParseUint(runnable, 10, 64)
	if err != nil {
		return agentTypes.LoadStat{}, fmt.Errorf("parse host runnable tasks: %w", err)
	}
	result.TotalTasks, err = strconv.ParseUint(total, 10, 64)
	if err != nil {
		return agentTypes.LoadStat{}, fmt.Errorf("parse host total tasks: %w", err)
	}
	return result, nil
}

func ParseMemory(data []byte) (agentTypes.MemoryStat, error) {
	values := make(map[string]uint64)
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err == nil {
			values[strings.TrimSuffix(fields[0], ":")] = value * 1024
		}
	}
	if values["MemTotal"] == 0 {
		return agentTypes.MemoryStat{}, errors.New("host memory data is empty")
	}
	return agentTypes.MemoryStat{
		Total:         values["MemTotal"],
		Available:     values["MemAvailable"],
		SwapTotal:     values["SwapTotal"],
		SwapAvailable: values["SwapFree"],
		Free:          memoryValue(values, "MemFree"),
		Buffers:       memoryValue(values, "Buffers"),
		Cached:        memoryValue(values, "Cached"),
		SwapCached:    memoryValue(values, "SwapCached"),
		Active:        memoryValue(values, "Active"),
		Inactive:      memoryValue(values, "Inactive"),
		ActiveAnon:    memoryValue(values, "Active(anon)"),
		InactiveAnon:  memoryValue(values, "Inactive(anon)"),
		ActiveFile:    memoryValue(values, "Active(file)"),
		InactiveFile:  memoryValue(values, "Inactive(file)"),
		AnonPages:     memoryValue(values, "AnonPages"),
		Mapped:        memoryValue(values, "Mapped"),
		Dirty:         memoryValue(values, "Dirty"),
		Writeback:     memoryValue(values, "Writeback"),
		Shmem:         memoryValue(values, "Shmem"),
		Slab:          memoryValue(values, "Slab"),
		SReclaimable:  memoryValue(values, "SReclaimable"),
		PageTables:    memoryValue(values, "PageTables"),
		KernelStack:   memoryValue(values, "KernelStack"),
	}, nil
}

func memoryValue(values map[string]uint64, name string) *uint64 {
	value, exists := values[name]
	if !exists {
		return nil
	}
	return &value
}

func ParsePressure(data []byte) (*agentTypes.PressureResource, error) {
	result := &agentTypes.PressureResource{}
	found := false
	var parseErr error
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			if len(fields) != 0 {
				parseErr = errors.New("host pressure data is incomplete")
			}
			continue
		}
		value := agentTypes.PressureValue{}
		valid := true
		seen := make(map[string]bool, 4)
		for _, field := range fields[1:] {
			key, raw, ok := strings.Cut(field, "=")
			if !ok {
				valid = false
				break
			}
			var err error
			switch key {
			case "avg10":
				value.Avg10, err = strconv.ParseFloat(raw, 64)
			case "avg60":
				value.Avg60, err = strconv.ParseFloat(raw, 64)
			case "avg300":
				value.Avg300, err = strconv.ParseFloat(raw, 64)
			case "total":
				value.Total, err = strconv.ParseUint(raw, 10, 64)
			default:
				continue
			}
			if err != nil {
				valid = false
				break
			}
			seen[key] = true
		}
		if !valid || !seen["avg10"] || !seen["avg60"] || !seen["avg300"] || !seen["total"] {
			parseErr = errors.New("parse host pressure data")
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
		if parseErr != nil {
			return nil, parseErr
		}
		return nil, errors.New("host pressure data is empty")
	}
	return result, parseErr
}
