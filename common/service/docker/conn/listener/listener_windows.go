//go:build windows

package listener

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"syscall"
	"time"
)
import winio "github.com/Microsoft/go-winio"

// New returns a nil listener when another process already serves this address.
func New(address string) (net.Listener, string, error) {
	pipeName, ok := strings.CutPrefix(address, "npipe:////./pipe/")
	if !ok || pipeName == "" {
		return nil, "", fmt.Errorf("invalid named pipe address %q", address)
	}
	pipePath := `\\.\pipe\` + pipeName
	conn, dialErr := winio.DialPipe(pipePath, pipeProbeTimeout())
	if dialErr == nil {
		_ = conn.Close()
		return nil, address, nil
	}
	if !errors.Is(dialErr, syscall.ERROR_FILE_NOT_FOUND) {
		return nil, "", fmt.Errorf("probe named pipe %s: %w", pipePath, dialErr)
	}

	listener, err := winio.ListenPipe(pipePath, &winio.PipeConfig{})
	if err != nil {
		if conn, dialErr := winio.DialPipe(pipePath, pipeProbeTimeout()); dialErr == nil {
			_ = conn.Close()
			return nil, address, nil
		}
		return nil, "", err
	}
	return listener, address, nil
}

func pipeProbeTimeout() *time.Duration {
	timeout := time.Second
	return &timeout
}
