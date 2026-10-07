//go:build !windows

package listener

import (
	"fmt"
	"net"
	"os"
	"strings"
)

func New(address string) (net.Listener, string, error) {
	sockPath, ok := strings.CutPrefix(address, "unix://")
	if !ok || sockPath == "" {
		return nil, "", fmt.Errorf("invalid Unix socket address %q", address)
	}
	_ = os.Remove(sockPath)
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: sockPath})
	if err != nil {
		return nil, "", err
	}

	return listener, address, nil
}
