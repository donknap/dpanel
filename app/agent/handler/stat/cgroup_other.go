//go:build !linux

package stat

import (
	"time"

	agentTypes "github.com/donknap/dpanel/app/agent/types"
)

type containerReader struct{}

func newContainerReader([]containerTarget) *containerReader {
	return &containerReader{}
}

func (*containerReader) Read(time.Time, cpuCounters, uint64) []agentTypes.ContainerStat {
	return nil
}
