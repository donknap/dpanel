//go:build linux

package check

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	agentTypes "github.com/donknap/dpanel/app/agent/types"
	"golang.org/x/sys/unix"
)

const (
	hostCgroupPath       = "/mnt_host_sys/fs/cgroup"
	inetDiagRequestSize  = 56
	inetDiagResponseSize = 72
	inetDiagCgroupID     = 21
	rtAttrHeaderSize     = 4
	tcpListenState       = 10
)

type socketInfo struct {
	cgroupID       uint64
	port           uint16
	protocol       string
	containerIndex int
}

func discoverPorts(ctx context.Context, result []agentTypes.PortCheckResult) error {
	targets := make([]int, 0, len(result))
	for index := range result {
		if len(result[index].Ports) == 0 {
			targets = append(targets, index)
		}
	}
	if len(targets) == 0 {
		return nil
	}

	if _, err := os.Stat(filepath.Join(hostCgroupPath, "cgroup.controllers")); err != nil {
		if os.IsNotExist(err) {
			setUnsupported(result, targets)
			return nil
		}
		return fmt.Errorf("inspect host cgroup: %w", err)
	}

	sockets, cgroupAttributeSeen, err := readHostSockets(ctx)
	if err != nil {
		return fmt.Errorf("query host sockets: %w", err)
	}
	if len(sockets) > 0 && !cgroupAttributeSeen {
		setUnsupported(result, targets)
		return nil
	}
	sort.Slice(sockets, func(i, j int) bool {
		if sockets[i].protocol != sockets[j].protocol {
			return sockets[i].protocol < sockets[j].protocol
		}
		if sockets[i].port != sockets[j].port {
			return sockets[i].port < sockets[j].port
		}
		return sockets[i].cgroupID < sockets[j].cgroupID
	})

	supported := make([]bool, len(result))
	err = filepath.WalkDir(hostCgroupPath, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err = ctx.Err(); err != nil {
			return err
		}
		if !entry.IsDir() {
			return nil
		}

		containerIndex := -1
		for _, targetIndex := range targets {
			if containsContainerID(path, result[targetIndex].ContainerID) {
				containerIndex = targetIndex
				break
			}
		}
		if containerIndex == -1 {
			return nil
		}

		handle, _, handleErr := unix.NameToHandleAt(unix.AT_FDCWD, path, 0)
		if errors.Is(handleErr, unix.ENOSYS) || errors.Is(handleErr, unix.EOPNOTSUPP) {
			setUnsupported(result, targets)
			return fs.SkipAll
		}
		if handleErr != nil {
			return fmt.Errorf("read cgroup handle %s: %w", path, handleErr)
		}
		if len(handle.Bytes()) != 8 {
			return fmt.Errorf("cgroup handle %s has unexpected size %d", path, len(handle.Bytes()))
		}

		supported[containerIndex] = true
		cgroupID := binary.NativeEndian.Uint64(handle.Bytes())
		for index := range sockets {
			if sockets[index].cgroupID == cgroupID {
				sockets[index].containerIndex = containerIndex
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("resolve container cgroups: %w", err)
	}
	for _, socket := range sockets {
		if socket.containerIndex == -1 || !supported[socket.containerIndex] {
			continue
		}
		value := fmt.Sprintf("%d/%s", socket.port, socket.protocol)
		found := false
		for _, item := range result[socket.containerIndex].Ports {
			if item.Port == value {
				found = true
				break
			}
		}
		if !found {
			result[socket.containerIndex].Ports = append(
				result[socket.containerIndex].Ports,
				agentTypes.PortCheckItem{Port: value},
			)
		}
	}
	for _, targetIndex := range targets {
		if len(result[targetIndex].Ports) != 0 {
			continue
		}
		status := portCheckNone
		if !supported[targetIndex] {
			status = portCheckUnsupported
		}
		result[targetIndex].Ports = append(result[targetIndex].Ports, agentTypes.PortCheckItem{
			Port:   "0",
			Status: status,
		})
	}
	return nil
}

func setUnsupported(result []agentTypes.PortCheckResult, targets []int) {
	for _, index := range targets {
		result[index].Ports = append(result[index].Ports, agentTypes.PortCheckItem{
			Port:   "0",
			Status: portCheckUnsupported,
		})
	}
}

func containsContainerID(cgroup, containerID string) bool {
	for offset := 0; offset < len(cgroup); {
		index := strings.Index(cgroup[offset:], containerID)
		if index == -1 {
			return false
		}
		index += offset
		beforeValid := index == 0 || !isLowerHex(cgroup[index-1])
		after := index + len(containerID)
		afterValid := after == len(cgroup) || !isLowerHex(cgroup[after])
		if beforeValid && afterValid {
			return true
		}
		offset = index + 1
	}
	return false
}

func isLowerHex(value byte) bool {
	return value >= '0' && value <= '9' || value >= 'a' && value <= 'f'
}

func readHostSockets(ctx context.Context) ([]socketInfo, bool, error) {
	fd, err := unix.Socket(unix.AF_NETLINK, unix.SOCK_DGRAM|unix.SOCK_CLOEXEC, unix.NETLINK_SOCK_DIAG)
	if err != nil {
		return nil, false, err
	}
	defer unix.Close(fd)
	if err = unix.Bind(fd, &unix.SockaddrNetlink{Family: unix.AF_NETLINK}); err != nil {
		return nil, false, err
	}

	result := make([]socketInfo, 0)
	cgroupAttributeSeen := false
	sequence := uint32(0)
	for _, protocol := range []struct {
		family   uint8
		protocol uint8
		states   uint32
		name     string
	}{
		{family: unix.AF_INET, protocol: unix.IPPROTO_TCP, states: 1 << tcpListenState, name: "tcp4"},
		{family: unix.AF_INET6, protocol: unix.IPPROTO_TCP, states: 1 << tcpListenState, name: "tcp6"},
		{family: unix.AF_INET, protocol: unix.IPPROTO_UDP, states: ^uint32(0), name: "udp4"},
		{family: unix.AF_INET6, protocol: unix.IPPROTO_UDP, states: ^uint32(0), name: "udp6"},
	} {
		if err = ctx.Err(); err != nil {
			return nil, false, err
		}
		sequence++
		request := make([]byte, unix.NLMSG_HDRLEN+inetDiagRequestSize)
		binary.NativeEndian.PutUint32(request[0:4], uint32(len(request)))
		binary.NativeEndian.PutUint16(request[4:6], unix.SOCK_DIAG_BY_FAMILY)
		binary.NativeEndian.PutUint16(request[6:8], unix.NLM_F_REQUEST|unix.NLM_F_DUMP)
		binary.NativeEndian.PutUint32(request[8:12], sequence)
		request[unix.NLMSG_HDRLEN] = protocol.family
		request[unix.NLMSG_HDRLEN+1] = protocol.protocol
		binary.NativeEndian.PutUint32(request[unix.NLMSG_HDRLEN+4:unix.NLMSG_HDRLEN+8], protocol.states)
		if err = unix.Sendto(fd, request, 0, &unix.SockaddrNetlink{Family: unix.AF_NETLINK}); err != nil {
			return nil, false, err
		}

		complete := false
		for !complete {
			buffer := make([]byte, 64*1024)
			length, _, receiveErr := unix.Recvfrom(fd, buffer, 0)
			if receiveErr != nil {
				return nil, false, receiveErr
			}
			messages, parseErr := syscall.ParseNetlinkMessage(buffer[:length])
			if parseErr != nil {
				return nil, false, parseErr
			}
			for _, message := range messages {
				if message.Header.Seq != sequence {
					continue
				}
				switch message.Header.Type {
				case unix.NLMSG_DONE:
					complete = true
				case unix.NLMSG_ERROR:
					if len(message.Data) < 4 {
						return nil, false, errors.New("short netlink error response")
					}
					errno := int32(binary.NativeEndian.Uint32(message.Data[:4]))
					if errno != 0 {
						return nil, false, syscall.Errno(-errno)
					}
				case unix.SOCK_DIAG_BY_FAMILY:
					if len(message.Data) < inetDiagResponseSize {
						return nil, false, errors.New("short socket diagnostic response")
					}
					cgroupID, found, parseErr := parseCgroupID(message.Data[inetDiagResponseSize:])
					if parseErr != nil {
						return nil, false, parseErr
					}
					cgroupAttributeSeen = cgroupAttributeSeen || found
					port := binary.BigEndian.Uint16(message.Data[4:6])
					if port == 0 ||
						protocol.protocol == unix.IPPROTO_TCP && message.Data[1] != tcpListenState ||
						protocol.protocol == unix.IPPROTO_UDP && binary.BigEndian.Uint16(message.Data[6:8]) != 0 {
						continue
					}
					result = append(result, socketInfo{
						cgroupID:       cgroupID,
						port:           port,
						protocol:       protocol.name,
						containerIndex: -1,
					})
				}
			}
		}
	}
	return result, cgroupAttributeSeen, nil
}

func parseCgroupID(attributes []byte) (uint64, bool, error) {
	for len(attributes) > 0 {
		if len(attributes) < rtAttrHeaderSize {
			return 0, false, errors.New("short socket diagnostic attribute")
		}
		length := int(binary.NativeEndian.Uint16(attributes[0:2]))
		if length < rtAttrHeaderSize || length > len(attributes) {
			return 0, false, errors.New("invalid socket diagnostic attribute length")
		}
		attributeType := binary.NativeEndian.Uint16(attributes[2:4]) & 0x3fff
		if attributeType == inetDiagCgroupID {
			if length < rtAttrHeaderSize+8 {
				return 0, false, errors.New("short socket cgroup id attribute")
			}
			return binary.NativeEndian.Uint64(attributes[rtAttrHeaderSize : rtAttrHeaderSize+8]), true, nil
		}
		alignedLength := (length + 3) &^ 3
		if alignedLength > len(attributes) {
			return 0, false, errors.New("invalid aligned socket diagnostic attribute length")
		}
		attributes = attributes[alignedLength:]
	}
	return 0, false, nil
}
