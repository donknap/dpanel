package usage

import (
	"context"
	"errors"
	"fmt"
	"runtime"

	"github.com/shirou/gopsutil/v4/disk"
)

const dockerDataMountPath = "/mnt_docker"

type Handler struct{}

type filesystemUsage struct {
	Used           uint64  `json:"used"`
	Available      uint64  `json:"available"`
	Total          uint64  `json:"total"`
	InodeUsed      *uint64 `json:"inodeUsed,omitempty"`
	InodeAvailable *uint64 `json:"inodeAvailable,omitempty"`
	InodeTotal     *uint64 `json:"inodeTotal,omitempty"`
}

func New() *Handler {
	return &Handler{}
}

func (*Handler) Handle(_ context.Context, args []string) (any, error) {
	if len(args) != 0 {
		return nil, errors.New("usage does not accept arguments")
	}
	if runtime.GOOS != "linux" {
		return nil, errors.New("filesystem usage is only supported on Linux")
	}

	value, err := disk.Usage(dockerDataMountPath)
	if err != nil {
		return nil, fmt.Errorf("read filesystem usage: %w", err)
	}
	result := filesystemUsage{Used: value.Used, Available: value.Free, Total: value.Total}
	if value.InodesTotal > 0 {
		result.InodeUsed = &value.InodesUsed
		result.InodeAvailable = &value.InodesFree
		result.InodeTotal = &value.InodesTotal
	}
	return result, nil
}
