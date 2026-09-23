package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	agentTypes "github.com/donknap/dpanel/app/agent/types"
	"github.com/donknap/dpanel/common/service/agent/runner"
)

const (
	portCheckSuccess     = "success"
	portCheckFailed      = "failed"
	portCheckTimeout     = "timeout"
	portCheckNone        = "none"
	portCheckUnsupported = "unsupported"

	statHeartbeatInterval = 3 * time.Second
)

type Agent struct {
	runner runner.Runner
	Fs     *Fs
}

func NewAgent(commandRunner runner.Runner) (*Agent, error) {
	if commandRunner == nil {
		return nil, errors.New("agent runner is required")
	}
	result := &Agent{runner: commandRunner}
	result.Fs = &Fs{agent: result}
	return result, nil
}

func (self *Agent) exec(ctx context.Context, result any, args ...string) error {
	if len(args) == 0 {
		return errors.New("agent command is empty")
	}
	output, execErr := self.runner.Run(ctx, args...)
	if len(output) == 0 && execErr != nil {
		return execErr
	}

	message := agentTypes.Message[json.RawMessage]{}
	if err := decodeJSON(output, &message); err != nil {
		return errors.Join(execErr, fmt.Errorf("decode agent response: %w", err))
	}
	if message.Code != 200 {
		if message.Error != "" {
			return errors.Join(execErr, errors.New(message.Error))
		}
		return errors.Join(execErr, fmt.Errorf("agent returned code %d", message.Code))
	}
	if message.Error != "" {
		return errors.Join(execErr, fmt.Errorf("agent returned an error with success code: %s", message.Error))
	}
	if execErr != nil {
		return execErr
	}
	if result == nil {
		return nil
	}
	if len(message.Data) == 0 || bytes.Equal(message.Data, []byte("null")) {
		return errors.New("agent response data is empty")
	}
	if err := decodeJSON(message.Data, result); err != nil {
		return fmt.Errorf("decode agent response data: %w", err)
	}
	return nil
}

func (self *Agent) CheckPorts(ctx context.Context, targets []PortCheckTarget) ([]agentTypes.PortCheckResult, error) {
	result := make([]agentTypes.PortCheckResult, 0, len(targets))
	if len(targets) == 0 {
		return result, nil
	}
	args := []string{"check", "--port"}
	for targetIndex, target := range targets {
		if target.ContainerID == "" {
			return nil, errors.New("invalid agent port target")
		}
		for previousIndex := 0; previousIndex < targetIndex; previousIndex++ {
			if targets[previousIndex].ContainerID == target.ContainerID {
				return nil, fmt.Errorf("duplicate agent port target %s", target.ContainerID)
			}
		}
		if len(target.Ports) == 0 {
			args = append(args, "--target", target.ContainerID)
			continue
		}
		for _, port := range target.Ports {
			args = append(args, "--target", target.ContainerID+":"+port)
		}
	}
	if err := self.exec(ctx, &result, args...); err != nil {
		return nil, fmt.Errorf("execute port check agent: %w", err)
	}
	if len(result) != len(targets) {
		return nil, errors.New("port check agent did not return all containers")
	}
	for index, target := range targets {
		if result[index].ContainerID != target.ContainerID {
			return nil, fmt.Errorf("port check agent returned unexpected container %s", result[index].ContainerID)
		}
		if result[index].Ports == nil {
			result[index].Ports = make([]agentTypes.PortCheckItem, 0)
		}
		for portIndex, port := range result[index].Ports {
			if port.Status == portCheckNone || port.Status == portCheckUnsupported {
				if port.Port != "0" || port.LatencyMillis != nil || len(result[index].Ports) != 1 {
					return nil, errors.New("port check agent returned invalid zero-port result")
				}
				continue
			}
			rawPort, protocol, found := strings.Cut(port.Port, "/")
			value, err := strconv.ParseUint(rawPort, 10, 16)
			if !found || strings.Contains(protocol, "/") || value == 0 || err != nil ||
				protocol != "tcp4" && protocol != "tcp6" && protocol != "udp4" && protocol != "udp6" {
				return nil, fmt.Errorf("port check agent returned invalid port %q", port.Port)
			}
			switch port.Status {
			case portCheckSuccess, portCheckFailed, portCheckTimeout:
			default:
				return nil, fmt.Errorf("port check agent returned invalid status %q", port.Status)
			}
			if port.LatencyMillis != nil && *port.LatencyMillis < 0 {
				return nil, fmt.Errorf("port check agent returned invalid latency %f", *port.LatencyMillis)
			}
			for previousIndex := 0; previousIndex < portIndex; previousIndex++ {
				if result[index].Ports[previousIndex].Port == port.Port {
					return nil, fmt.Errorf("port check agent returned duplicate port %q", port.Port)
				}
			}
		}
	}
	return result, nil
}

func (self *Agent) Usage(ctx context.Context) (agentTypes.FilesystemUsage, error) {
	result := agentTypes.FilesystemUsage{}
	if err := self.exec(ctx, &result, "usage"); err != nil {
		return agentTypes.FilesystemUsage{}, fmt.Errorf("execute agent usage: %w", err)
	}
	return result, nil
}

func (self *Agent) StreamStat(
	ctx context.Context, targets []ContainerTarget, handle func([]byte) error,
) error {
	if handle == nil {
		return errors.New("agent stat handler is required")
	}
	args := []string{"stat"}
	for _, target := range targets {
		if target.ContainerID == "" || target.PID <= 1 {
			return errors.New("invalid agent container target")
		}
		args = append(args, "--container-id", fmt.Sprintf("%s:%d", target.ContainerID, target.PID))
	}
	session, err := self.runner.Stream(ctx, args...)
	if err != nil {
		return err
	}
	var writeMu sync.Mutex
	writeCommand := func(command string) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		_, err := session.Write([]byte(command + "\n"))
		return err
	}
	var closeOnce sync.Once
	closeSession := func() {
		closeOnce.Do(func() {
			_ = writeCommand("close")
			_ = session.CloseWrite()
			_ = session.Close()
		})
	}
	stopClose := context.AfterFunc(ctx, closeSession)
	defer stopClose()
	defer closeSession()
	if err = writeCommand("ping"); err != nil {
		return fmt.Errorf("start agent stat heartbeat: %w", err)
	}

	heartbeatErr := make(chan error, 1)
	heartbeatDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(statHeartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-heartbeatDone:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := writeCommand("ping"); err != nil {
					select {
					case heartbeatErr <- err:
					default:
					}
					_ = session.Close()
					return
				}
			}
		}
	}()
	defer close(heartbeatDone)

	scanner := bufio.NewScanner(session)
	scanner.Buffer(make([]byte, 64*1024), 64*1024*1024)
	for scanner.Scan() {
		data := bytes.Clone(scanner.Bytes())
		if err = decodeStreamError(data); err != nil {
			return err
		}
		if err = handle(data); err != nil {
			return err
		}
	}
	select {
	case err = <-heartbeatErr:
		return fmt.Errorf("write agent stat heartbeat: %w", err)
	default:
	}
	if err = scanner.Err(); err != nil && ctx.Err() == nil {
		return fmt.Errorf("read agent stat: %w", err)
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return nil
}

func decodeJSON(data []byte, result any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(result); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return errors.New("response contains trailing data")
	}
	return nil
}

func decodeLines[T any](data []byte, handle func(T) error) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	for {
		var response T
		if err := decoder.Decode(&response); errors.Is(err, io.EOF) {
			return nil
		} else if err != nil {
			return err
		}
		if err := handle(response); err != nil {
			return err
		}
	}
}

func decodeStreamError(data []byte) error {
	var header struct {
		Code  *int   `json:"code"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(data, &header); err != nil || header.Code == nil {
		return nil
	}
	if header.Error != "" {
		return errors.New(header.Error)
	}
	return fmt.Errorf("agent returned code %d", *header.Code)
}
