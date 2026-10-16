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
		conn, err := net.DialTimeout("unix", sockPath, time.Second)
		if err == nil {
			_ = conn.Close()
			return nil, address, nil
		}
		if !errors.Is(err, syscall.ECONNREFUSED) && !errors.Is(err, os.ErrNotExist) {
			return nil, "", fmt.Errorf("probe Unix socket %s: %w", sockPath, err)
		}
		if current, err := os.Lstat(sockPath); err == nil && os.SameFile(info, current) {
			if err := os.Remove(sockPath); err != nil {
				return nil, "", err
			}
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, "", err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, "", err
	}
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: sockPath})
	if err != nil {
		// Another process may have bound the address after the probe.
		if conn, err := net.DialTimeout("unix", sockPath, time.Second); err == nil {
			_ = conn.Close()
			return nil, address, nil
		}
		return nil, "", err
	}

	return listener, address, nil
}
