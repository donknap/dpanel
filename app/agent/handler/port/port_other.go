//go:build !linux

package port

import (
	"context"
	"errors"

	agentTypes "github.com/donknap/dpanel/app/agent/types"
)

func readPorts(context.Context, containerTarget) ([]agentTypes.Port, error) {
	return nil, errors.New("port detection is only supported on Linux")
}
