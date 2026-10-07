//go:build windows

package listener

import (
	"fmt"
	"net"
	"strings"
)
import winio "github.com/Microsoft/go-winio"

func New(address string) (net.Listener, string, error) {
	pipeName, ok := strings.CutPrefix(address, "npipe:////./pipe/")
	if !ok || pipeName == "" {
		return nil, "", fmt.Errorf("invalid named pipe address %q", address)
	}
	pipePath := `\\.\pipe\` + pipeName

	listener, err := winio.ListenPipe(pipePath, &winio.PipeConfig{})
	if err != nil {
		return nil, "", err
	}
	return listener, address, nil
}
