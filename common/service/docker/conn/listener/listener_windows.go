//go:build windows

package listener

import "net"
import winio "github.com/Microsoft/go-winio"

func New(sockPath string) (net.Listener, string, error) {
	address := "npipe:////./pipe/" + sockPath
	listener, err := winio.ListenPipe(address, &winio.PipeConfig{})
	if err != nil {
		return nil, "", err
	}
	return listener, address, nil
}
