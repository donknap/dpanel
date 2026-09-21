package port

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	agentTypes "github.com/donknap/dpanel/app/agent/types"
)

type Handler struct{}

type containerTarget struct {
	id  string
	pid int
}

const portWorkerTotal = 4

func New() *Handler {
	return &Handler{}
}

func (*Handler) Handle(ctx context.Context, args []string) (any, error) {
	result := make([]agentTypes.PortResult, 0)
	err := (&Handler{}).HandleStream(ctx, args, func(value any) error {
		result = append(result, value.(agentTypes.PortResult))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (*Handler) HandleStream(ctx context.Context, args []string, emit func(any) error) error {
	targets, err := parseTargets(args)
	if err != nil {
		return err
	}
	jobs := make(chan containerTarget)
	results := make(chan agentTypes.PortResult, len(targets))
	workerTotal := min(portWorkerTotal, len(targets))
	var workers sync.WaitGroup
	workers.Add(workerTotal)
	for range workerTotal {
		go func() {
			defer workers.Done()
			for target := range jobs {
				ports, readErr := readPorts(ctx, target)
				result := agentTypes.PortResult{
					ContainerID: target.id,
					Ports:       make([]agentTypes.Port, 0),
				}
				if readErr != nil {
					result.Error = readErr.Error()
				} else {
					result.Ports = ports
				}
				results <- result
			}
		}()
	}
	go func() {
		defer close(jobs)
		for _, target := range targets {
			select {
			case jobs <- target:
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() {
		workers.Wait()
		close(results)
	}()
	completed := 0
	for result := range results {
		if err = emit(result); err != nil {
			return err
		}
		completed++
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if completed != len(targets) {
		return errors.New("port detection did not complete all containers")
	}
	return nil
}

func parseTargets(args []string) ([]containerTarget, error) {
	if len(args) == 0 || len(args)%2 != 0 {
		return nil, errors.New("expected repeated --container-id <64-char-id>:<host-pid>")
	}
	result := make([]containerTarget, 0, len(args)/2)
	seen := make(map[string]struct{}, len(args)/2)
	for index := 0; index < len(args); index += 2 {
		if args[index] != "--container-id" {
			return nil, errors.New("expected repeated --container-id <64-char-id>:<host-pid>")
		}
		target, err := parseTarget(args[index+1])
		if err != nil {
			return nil, err
		}
		if _, exists := seen[target.id]; exists {
			return nil, fmt.Errorf("duplicate container id %q", target.id)
		}
		seen[target.id] = struct{}{}
		result = append(result, target)
	}
	return result, nil
}

func parseTarget(value string) (containerTarget, error) {
	fields := strings.Split(value, ":")
	if len(fields) != 2 || len(fields[0]) != 64 || strings.ToLower(fields[0]) != fields[0] {
		return containerTarget{}, fmt.Errorf("invalid --container-id value %q, expected <64-char-id>:<host-pid>", value)
	}
	if _, err := hex.DecodeString(fields[0]); err != nil {
		return containerTarget{}, fmt.Errorf("invalid --container-id value %q, expected <64-char-id>:<host-pid>", value)
	}
	rawPID := fields[1]
	if rawPID == "" || rawPID[0] == '+' || len(rawPID) > 1 && rawPID[0] == '0' {
		return containerTarget{}, fmt.Errorf("invalid host pid in --container-id value %q", value)
	}
	pid, err := strconv.ParseInt(rawPID, 10, 32)
	if err != nil || pid <= 1 {
		return containerTarget{}, fmt.Errorf("invalid host pid in --container-id value %q", value)
	}
	return containerTarget{id: fields[0], pid: int(pid)}, nil
}
