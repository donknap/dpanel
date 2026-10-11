//go:build !windows

package listener

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"syscall"
	"time"
)

// New returns a nil listener when another process already serves this address.
func New(address string) (net.Listener, string, error) {
	sockPath, ok := strings.CutPrefix(address, "unix://")
	if !ok || sockPath == "" {
		return nil, "", fmt.Errorf("invalid Unix socket address %q", address)
	}
	if info, err := os.Lstat(sockPath); err == nil {
		if info.Mode()&os.ModeSocket == 0 {
			return nil, "", fmt.Errorf("%s exists and is not a socket", sockPath)
		}
		conn, dialErr := net.DialTimeout("unix", sockPath, time.Second)
		if dialErr == nil {
			_ = conn.Close()
			return nil, address, nil
		}
		if !errors.Is(dialErr, syscall.ECONNREFUSED) && !errors.Is(dialErr, os.ErrNotExist) {
			return nil, "", fmt.Errorf("probe Unix socket %s: %w", sockPath, dialErr)
		}
		if current, statErr := os.Lstat(sockPath); statErr == nil && os.SameFile(info, current) {
			if err := os.Remove(sockPath); err != nil {
				return nil, "", err
			}
		} else if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
			return nil, "", statErr
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, "", err
	}
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: sockPath})
	if err != nil {
		// Another process may have bound the address after the probe.
		if conn, dialErr := net.DialTimeout("unix", sockPath, time.Second); dialErr == nil {
			_ = conn.Close()
			return nil, address, nil
		}
		return nil, "", err
	}

	return listener, address, nil
}
