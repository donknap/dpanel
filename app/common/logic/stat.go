package logic

import (
	"encoding/json"
	"github.com/docker/docker/api/types/system"
	"github.com/docker/go-units"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/exec"
	"math"
	"strconv"
	"strings"
	"time"
)

type Stat struct {
}

type statItemResult struct {
	Cpu       float64      `json:"cpu"`
	Memory    ioItemResult `json:"memory"`
	BlockIO   ioItemResult `json:"blockIO"`
	NetworkIO ioItemResult `json:"networkIO"`
	Name      string       `json:"name"`
	Container string       `json:"container"`
}

type ioItemResult struct {
	In  int64 `json:"in"`
	Out int64 `json:"out"`
}

func (self Stat) GetCommandResult() string {
	option := docker.Sdk.GetRunCmd(
		"stats", "-a",
		"--format", "json", "--no-stream")
	option = append(option, exec.WithTimeout(time.Minute*10))
	if cmd, err := exec.New(option...); err == nil {
		return cmd.RunWithResult()
	}
	return ""
}

func (self Stat) GetStat(dockerInfo system.Info, response string) ([]*statItemResult, error) {
	result := make([]*statItemResult, 0)
	statJsonItem := struct {
		BlockIO   string
		CPUPerc   string
		MemPerc   string
		MemUsage  string
		NetIO     string
		Name      string
		Container string
	}{}
	if response == "" {
		return result, nil
	}
	for _, line := range strings.Split(response, "\n") {
		if line == "" || !strings.Contains(line, "\"Name\":") {
			continue
		}
		err := json.Unmarshal([]byte(line), &statJsonItem)
		if err != nil {
			return nil, err
		}
		r := &statItemResult{
			Name:      statJsonItem.Name,
			Container: statJsonItem.Container,
		}
		cpu, _ := strconv.ParseFloat(strings.TrimSuffix(statJsonItem.CPUPerc, "%"), 64)
		// 使用率超过100%时，代表该容器使用超过1核。需要将占用转换成100%之内的占用
		if cpu > 100 {
			cpu = math.Round(cpu/float64(dockerInfo.NCPU)*100) / 100
		}
		r.Cpu += cpu
		if strings.Contains(statJsonItem.MemUsage, "/") {
			memory := strings.Split(statJsonItem.MemUsage, "/")
			use, _ := units.RAMInBytes(strings.TrimSpace(memory[0]))
			limit, _ := units.RAMInBytes(strings.TrimSpace(memory[1]))

			r.Memory.In = use
			r.Memory.Out = limit
		}
		if strings.Contains(statJsonItem.NetIO, "/") {
			networkIo := strings.Split(statJsonItem.NetIO, "/")
			in, _ := units.RAMInBytes(strings.TrimSpace(networkIo[0]))
			out, _ := units.RAMInBytes(strings.TrimSpace(networkIo[1]))

			r.NetworkIO.In = in
			r.NetworkIO.Out = out
		}
		if strings.Contains(statJsonItem.BlockIO, "/") {
			blockIo := strings.Split(statJsonItem.BlockIO, "/")
			in, _ := units.RAMInBytes(strings.TrimSpace(blockIo[0]))
			out, _ := units.RAMInBytes(strings.TrimSpace(blockIo[1]))

			r.BlockIO.In = in
			r.BlockIO.Out = out
		}
		result = append(result, r)
	}
	return result, nil
}
