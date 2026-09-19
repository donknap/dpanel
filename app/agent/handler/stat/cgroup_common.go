package stat

import (
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

const maxContainerIDOption = 4096

type statOptions struct {
	containers []containerTarget
}

type containerTarget struct {
	id  string
	pid int
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
