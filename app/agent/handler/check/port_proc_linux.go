//go:build linux

package check

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	agentTypes "github.com/donknap/dpanel/app/agent/types"
)

const hostProcPath = "/mnt_host_proc"

type procSocket struct {
	port     uint16
	protocol string
}

func discoverPortsProc(ctx context.Context, result []agentTypes.PortCheckResult, targets []int) error {
	// The application only auto-discovers ports for host-network containers.
	// Bridge containers use published host ports. The monitor also uses host
	// networking, so the host /proc/net contains the sockets needed here.
	sockets := make(map[uint64]procSocket)
	var readErrors []agentTypes.PortCheckError
	for _, protocol := range []string{"tcp", "tcp6", "udp", "udp6"} {
		if err := readProcSockets(filepath.Join(hostProcPath, "net", protocol), protocol, sockets); err != nil {
			readErrors = append(readErrors, agentTypes.PortCheckError{
				Check: protocol, Status: portCheckFailed, Error: fmt.Sprintf("read host /proc/net/%s: %v", protocol, err),
			})
		}
	}
	for _, index := range targets {
		result[index].Errors = append(result[index].Errors, readErrors...)
	}
	if len(readErrors) == 4 {
		return nil
	}

	entries, err := os.ReadDir(hostProcPath)
	if err != nil {
		for _, index := range targets {
			result[index].Errors = append(result[index].Errors, agentTypes.PortCheckError{
				Check: "proc", Status: portCheckFailed, Error: fmt.Sprintf("list host processes: %v", err),
			})
		}
		return nil
	}

	foundProcess := make([]bool, len(result))
	fdErrors := make([]error, len(result))
	var cgroupError error
	for _, entry := range entries {
		if err = ctx.Err(); err != nil {
			return err
		}
		if _, err = strconv.Atoi(entry.Name()); err != nil {
			continue
		}
		pidPath := filepath.Join(hostProcPath, entry.Name())
		cgroup, err := os.ReadFile(filepath.Join(pidPath, "cgroup"))
		if err != nil {
			if !os.IsNotExist(err) && cgroupError == nil {
				cgroupError = fmt.Errorf("read host process %s cgroup: %w", entry.Name(), err)
			}
			continue
		}
		for _, index := range targets {
			if !containsContainerID(string(cgroup), result[index].ContainerID) {
				continue
			}
			foundProcess[index] = true
			fds, err := os.ReadDir(filepath.Join(pidPath, "fd"))
			if err != nil {
				if fdErrors[index] == nil {
					fdErrors[index] = fmt.Errorf("list host process %s fd: %w", entry.Name(), err)
				}
				continue
			}
			for _, fd := range fds {
				link, err := os.Readlink(filepath.Join(pidPath, "fd", fd.Name()))
				if err != nil {
					if !os.IsNotExist(err) && fdErrors[index] == nil {
						fdErrors[index] = fmt.Errorf("read host process %s fd %s: %w", entry.Name(), fd.Name(), err)
					}
					continue
				}
				if !strings.HasPrefix(link, "socket:[") || !strings.HasSuffix(link, "]") {
					continue
				}
				inode, err := strconv.ParseUint(link[len("socket:["):len(link)-1], 10, 64)
				if err != nil {
					continue
				}
				socket, ok := sockets[inode]
				if !ok {
					continue
				}
				value := fmt.Sprintf("%d/%s", socket.port, socket.protocol)
				seen := false
				for _, port := range result[index].Ports {
					if port.Port == value {
						seen = true
						break
					}
				}
				if !seen {
					result[index].Ports = append(result[index].Ports, agentTypes.PortCheckItem{Port: value})
				}
			}
		}
	}
	for _, index := range targets {
		if !foundProcess[index] && cgroupError != nil {
			result[index].Errors = append(result[index].Errors, agentTypes.PortCheckError{
				Check: "proc", Status: portCheckFailed, Error: cgroupError.Error(),
			})
		}
		if fdErrors[index] != nil {
			result[index].Errors = append(result[index].Errors, agentTypes.PortCheckError{
				Check: "proc", Status: portCheckFailed, Error: fdErrors[index].Error(),
			})
		}
		if len(result[index].Ports) != 0 || len(result[index].Errors) != 0 {
			continue
		}
		status := portCheckNone
		if !foundProcess[index] {
			status = portCheckUnsupported
		}
		result[index].Ports = append(result[index].Ports, agentTypes.PortCheckItem{Port: "0", Status: status})
	}
	return nil
}

func readProcSockets(path, protocol string, sockets map[uint64]procSocket) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 10 || fields[0] == "sl" {
			continue
		}
		if strings.HasPrefix(protocol, "tcp") && fields[3] != "0A" {
			continue
		}
		if strings.HasPrefix(protocol, "udp") && !strings.HasSuffix(fields[2], ":0000") {
			continue
		}
		_, rawPort, ok := strings.Cut(fields[1], ":")
		if !ok {
			continue
		}
		port, err := strconv.ParseUint(rawPort, 16, 16)
		if err != nil || port == 0 {
			continue
		}
		inode, err := strconv.ParseUint(fields[9], 10, 64)
		if err != nil || inode == 0 {
			continue
		}
		name := protocol
		if !strings.HasSuffix(name, "6") {
			name += "4"
		}
		sockets[inode] = procSocket{port: uint16(port), protocol: name}
	}
	return scanner.Err()
}
