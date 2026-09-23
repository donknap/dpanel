package check

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	agentTypes "github.com/donknap/dpanel/app/agent/types"
)

type portCheckJob struct {
	containerIndex int
	portIndex      int
}

const (
	portCheckWorkerTotal     = 16
	portCheckTimeoutDuration = 3 * time.Second

	portCheckSuccess     = "success"
	portCheckFailed      = "failed"
	portCheckTimeout     = "timeout"
	portCheckNone        = "none"
	portCheckUnsupported = "unsupported"
)

func checkPorts(ctx context.Context, args []string) (any, error) {
	result, err := parseTargets(args)
	if err != nil {
		return nil, err
	}
	if err = discoverPorts(ctx, result); err != nil {
		return nil, err
	}

	jobs := make(chan portCheckJob)
	total := 0
	for _, target := range result {
		for _, port := range target.Ports {
			if port.Status == "" {
				total++
			}
		}
	}
	workerTotal := min(portCheckWorkerTotal, total)
	var workers sync.WaitGroup
	workers.Add(workerTotal)
	for range workerTotal {
		go func() {
			defer workers.Done()
			for job := range jobs {
				item := &result[job.containerIndex].Ports[job.portIndex]
				*item = checkPort(ctx, item.Port)
			}
		}()
	}
	for containerIndex := range result {
		for portIndex := range result[containerIndex].Ports {
			if result[containerIndex].Ports[portIndex].Status != "" {
				continue
			}
			select {
			case jobs <- portCheckJob{containerIndex: containerIndex, portIndex: portIndex}:
			case <-ctx.Done():
				close(jobs)
				workers.Wait()
				return nil, ctx.Err()
			}
		}
	}
	close(jobs)
	workers.Wait()
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func parseTargets(args []string) ([]agentTypes.PortCheckResult, error) {
	if len(args) == 0 || len(args)%2 != 0 {
		return nil, errors.New("expected repeated --target <container-id>[:<port>/<protocol>]")
	}
	result := make([]agentTypes.PortCheckResult, 0, len(args)/2)
	for index := 0; index < len(args); index += 2 {
		if args[index] != "--target" {
			return nil, errors.New("expected repeated --target <container-id>[:<port>/<protocol>]")
		}
		containerID, port, found := strings.Cut(args[index+1], ":")
		if len(containerID) != 64 || strings.ToLower(containerID) != containerID {
			return nil, fmt.Errorf("invalid container id %q", containerID)
		}
		if _, err := hex.DecodeString(containerID); err != nil {
			return nil, fmt.Errorf("invalid container id %q", containerID)
		}

		containerIndex := -1
		for resultIndex := range result {
			if result[resultIndex].ContainerID == containerID {
				containerIndex = resultIndex
				break
			}
		}
		if containerIndex == -1 {
			result = append(result, agentTypes.PortCheckResult{
				ContainerID: containerID,
				Ports:       make([]agentTypes.PortCheckItem, 0),
			})
			containerIndex = len(result) - 1
		}
		if !found {
			continue
		}
		value, err := parsePort(port)
		if err != nil {
			return nil, err
		}
		duplicate := false
		for _, item := range result[containerIndex].Ports {
			if item.Port == value {
				duplicate = true
				break
			}
		}
		if !duplicate {
			result[containerIndex].Ports = append(result[containerIndex].Ports, agentTypes.PortCheckItem{Port: value})
		}
	}
	return result, nil
}

func parsePort(value string) (string, error) {
	rawPort, protocol, found := strings.Cut(value, "/")
	protocol = strings.ToLower(protocol)
	if !found || strings.Contains(protocol, "/") ||
		protocol != "tcp4" && protocol != "tcp6" && protocol != "udp4" && protocol != "udp6" {
		return "", fmt.Errorf("invalid port %q, expected <port>/<tcp4|tcp6|udp4|udp6>", value)
	}
	if rawPort == "" || rawPort[0] == '+' || len(rawPort) > 1 && rawPort[0] == '0' {
		return "", fmt.Errorf("invalid port %q, expected <port>/<tcp4|tcp6|udp4|udp6>", value)
	}
	port, err := strconv.ParseUint(rawPort, 10, 16)
	if err != nil || port == 0 {
		return "", fmt.Errorf("invalid port %q, expected <port>/<tcp4|tcp6|udp4|udp6>", value)
	}
	return fmt.Sprintf("%d/%s", port, protocol), nil
}

func checkPort(ctx context.Context, value string) agentTypes.PortCheckItem {
	result := agentTypes.PortCheckItem{Port: value}
	rawPort, protocol, _ := strings.Cut(value, "/")
	host := "127.0.0.1"
	if strings.HasSuffix(protocol, "6") {
		host = "::1"
	}
	startedAt := time.Now()
	checkCtx, cancel := context.WithTimeout(ctx, portCheckTimeoutDuration)
	defer cancel()
	connection, err := (&net.Dialer{}).DialContext(
		checkCtx,
		protocol,
		net.JoinHostPort(host, rawPort),
	)
	if err == nil && strings.HasPrefix(protocol, "udp") {
		deadline, ok := checkCtx.Deadline()
		if ok {
			err = connection.SetDeadline(deadline)
		}
		if err == nil {
			_, err = connection.Write([]byte{0})
		}
		if err == nil {
			buffer := make([]byte, 1)
			_, err = connection.Read(buffer)
		}
	}
	if connection != nil {
		_ = connection.Close()
	}
	latencyMillis := float64(time.Since(startedAt)) / float64(time.Millisecond)
	result.LatencyMillis = &latencyMillis
	if err == nil {
		result.Status = portCheckSuccess
		return result
	}
	var netErr net.Error
	if errors.Is(err, context.DeadlineExceeded) || errors.As(err, &netErr) && netErr.Timeout() {
		result.Status = portCheckTimeout
		return result
	}
	result.Status = portCheckFailed
	return result
}
