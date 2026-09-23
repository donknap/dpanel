//go:build !linux

package port

import (
	"context"

	agentTypes "github.com/donknap/dpanel/app/agent/types"
)

func discoverPorts(context.Context, []agentTypes.PortCheckResult) error {
	return nil
}
