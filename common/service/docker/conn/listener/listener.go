//go:build !windows

package listener

import (
	"net"
)

func New(sockPath string) (net.Listener, string, error) {
	address := "unix://" + sockPath
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: sockPath})
	if err != nil {
		return nil, "", err
	}

	return listener, address, nil
}
