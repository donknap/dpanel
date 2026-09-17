package usage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"runtime"
	"strings"

	"github.com/shirou/gopsutil/v4/disk"
)

type Handler struct{}

type options struct {
	root string
	path string
}

type filesystemUsage struct {
	Used      uint64 `json:"used"`
	Available uint64 `json:"available"`
	Total     uint64 `json:"total"`
}

func New() *Handler {
	return &Handler{}
}

func (*Handler) Handle(_ context.Context, args []string) (any, error) {
	option, err := parseOptions(args)
	if err != nil {
		return nil, err
	}
	if runtime.GOOS != "linux" {
		return nil, errors.New("filesystem usage is only supported on Linux")
	}

	root, err := os.OpenRoot(option.root)
	if err != nil {
		return nil, fmt.Errorf("open root: %w", err)
	}
	defer root.Close()

	relativePath := strings.TrimPrefix(option.path, "/")
	if relativePath == "" {
		relativePath = "."
	}
	target, err := root.Open(relativePath)
	if err != nil {
		return nil, fmt.Errorf("open usage path: %w", err)
	}
	defer target.Close()

	value, err := disk.Usage(fmt.Sprintf("/proc/self/fd/%d", target.Fd()))
	if err != nil {
		return nil, fmt.Errorf("read filesystem usage: %w", err)
	}
	return filesystemUsage{
		Used:      value.Used,
		Available: value.Free,
		Total:     value.Total,
	}, nil
}

func parseOptions(args []string) (options, error) {
	option := options{}
	values := make(map[string]string)
	for len(args) > 0 {
		name := args[0]
		args = args[1:]
		if name != "--root" && name != "--path" {
			return option, fmt.Errorf("unknown option: %s", name)
		}
		if _, ok := values[name]; ok {
			return option, fmt.Errorf("duplicate option: %s", name)
		}
		if len(args) == 0 {
			return option, fmt.Errorf("option %s requires a value", name)
		}
		values[name] = args[0]
		args = args[1:]
	}

	if err := validateAbsolutePath("root", values["--root"]); err != nil {
		return option, err
	}
	if values["--root"] == "/proc" || strings.HasPrefix(values["--root"], "/proc/") {
		return option, errors.New("option --root must not point to /proc")
	}
	if err := validateAbsolutePath("path", values["--path"]); err != nil {
		return option, err
	}
	option.root = values["--root"]
	option.path = values["--path"]
	return option, nil
}

func validateAbsolutePath(name, value string) error {
	if value == "" {
		return fmt.Errorf("option --%s is required", name)
	}
	if strings.IndexByte(value, 0) >= 0 {
		return fmt.Errorf("%s contains a null byte", name)
	}
	if !strings.HasPrefix(value, "/") {
		return fmt.Errorf("%s must be absolute", name)
	}
	if path.Clean(value) != value {
		return fmt.Errorf("%s must be canonical", name)
	}
	return nil
}
