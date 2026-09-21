//go:build linux

package port

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	agentTypes "github.com/donknap/dpanel/app/agent/types"
)

const (
	hostProcPath   = "/mnt_host_proc"
	hostCgroupPath = "/mnt_host_sys/fs/cgroup"
)

func readPorts(ctx context.Context, target containerTarget) ([]agentTypes.Port, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	pids, err := targetPIDs(target)
	if err != nil {
		return nil, fmt.Errorf("locate container %s processes: %w", target.id, err)
	}
	ports := make(map[string]agentTypes.Port)
	for _, pid := range pids {
		if err = readProcessPorts(pid, ports); err != nil {
			return nil, fmt.Errorf("read container %s process %d sockets: %w", target.id, pid, err)
		}
	}
	result := make([]agentTypes.Port, 0, len(ports))
	for _, item := range ports {
		result = append(result, item)
	}
	return result, nil
}

func targetPIDs(target containerTarget) ([]int, error) {
	data, err := os.ReadFile(filepath.Join(hostProcPath, strconv.Itoa(target.pid), "cgroup"))
	if err != nil {
		return nil, err
	}
	legacy := make([]struct{ controllers, path string }, 0)
	unified := ""
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		fields := strings.SplitN(line, ":", 3)
		if len(fields) != 3 || !filepath.IsAbs(fields[2]) || filepath.Clean(fields[2]) != fields[2] {
			return nil, fmt.Errorf("invalid cgroup entry %q", line)
		}
		if fields[2] == "/" {
			return nil, errors.New("refusing to scan the host cgroup")
		}
		if fields[1] == "" {
			unified = fields[2]
		} else {
			legacy = append(legacy, struct{ controllers, path string }{fields[1], fields[2]})
		}
	}
	directory := ""
	if unified != "" {
		directory = filepath.Join(hostCgroupPath, unified)
	} else {
		directory = legacyCgroupDirectory(legacy)
	}
	if directory == "" {
		return nil, errors.New("container cgroup directory was not found")
	}
	pids := make(map[int]struct{})
	err = filepath.WalkDir(directory, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || entry.Name() != "cgroup.procs" {
			return nil
		}
		value, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		for _, field := range strings.Fields(string(value)) {
			pid, parseErr := strconv.Atoi(field)
			if parseErr != nil || pid < 0 || pid == 1 {
				return fmt.Errorf("invalid pid %q in %s", field, path)
			}
			if pid == 0 {
				continue
			}
			pids[pid] = struct{}{}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if _, exists := pids[target.pid]; !exists {
		return nil, errors.New("target pid is no longer in its cgroup")
	}
	result := make([]int, 0, len(pids))
	for pid := range pids {
		result = append(result, pid)
	}
	return result, nil
}

func legacyCgroupDirectory(entries []struct{ controllers, path string }) string {
	preferred := []string{"pids", "memory", "cpuacct", "cpu"}
	for _, controller := range preferred {
		for _, entry := range entries {
			if !containsController(entry.controllers, controller) {
				continue
			}
			for _, mount := range []string{controller, entry.controllers} {
				directory := filepath.Join(hostCgroupPath, mount, entry.path)
				if info, err := os.Stat(directory); err == nil && info.IsDir() {
					return directory
				}
			}
		}
	}
	return ""
}

func containsController(value, target string) bool {
	for _, item := range strings.Split(value, ",") {
		if item == target {
			return true
		}
	}
	return false
}

func readProcessPorts(pid int, result map[string]agentTypes.Port) error {
	fds, err := os.ReadDir(filepath.Join(hostProcPath, strconv.Itoa(pid), "fd"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	inodes := make(map[string]struct{})
	for _, fd := range fds {
		target, readErr := os.Readlink(filepath.Join(hostProcPath, strconv.Itoa(pid), "fd", fd.Name()))
		if os.IsNotExist(readErr) {
			continue
		}
		if readErr != nil {
			return readErr
		}
		if strings.HasPrefix(target, "socket:[") && strings.HasSuffix(target, "]") {
			inodes[strings.TrimSuffix(strings.TrimPrefix(target, "socket:["), "]")] = struct{}{}
		}
	}
	if len(inodes) == 0 {
		return nil
	}
	for _, table := range []string{"tcp", "tcp6"} {
		if err = readSocketTable(filepath.Join(hostProcPath, strconv.Itoa(pid), "net", table), inodes, result); err != nil {
			return err
		}
	}
	return nil
}

func readSocketTable(name string, inodes map[string]struct{}, result map[string]agentTypes.Port) error {
	file, err := os.Open(name)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 10 || fields[0] == "sl" || fields[3] != "0A" {
			continue
		}
		if _, exists := inodes[fields[9]]; !exists {
			continue
		}
		_, rawPort, found := strings.Cut(fields[1], ":")
		if !found {
			return fmt.Errorf("invalid local address %q", fields[1])
		}
		port, parseErr := strconv.ParseUint(rawPort, 16, 16)
		if parseErr != nil {
			return fmt.Errorf("parse local port %q: %w", rawPort, parseErr)
		}
		if port == 0 {
			continue
		}
		key := fmt.Sprintf("tcp:%d", port)
		result[key] = agentTypes.Port{Port: uint16(port), Protocol: "tcp"}
	}
	return scanner.Err()
}
