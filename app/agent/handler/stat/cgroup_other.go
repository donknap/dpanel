//go:build !linux

package stat

import "time"

type containerReader struct{}

func newContainerReader([]containerTarget) *containerReader {
	return &containerReader{}
}

func (*containerReader) Read(time.Time, cpuCounters, uint64) []containerStat {
	return nil
}
