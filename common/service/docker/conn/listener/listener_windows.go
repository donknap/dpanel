//go:build windows

package listener

import (
	"net"
	"time"
)
import winio "github.com/Microsoft/go-winio"

func New(sockPath string) (net.Listener, string, error) {
	address := `\\.\pipe\` + sockPath

	cleanupOldPipe(address)

	listener, err := winio.ListenPipe(address, &winio.PipeConfig{})
	if err != nil {
		return nil, "", err
	}
	return listener, `npipe:////./pipe/` + sockPath, nil
}

func cleanupOldPipe(address string) {
	conn, err := winio.DialPipe(address, nil)
	if err != nil {
		return
	}
	err = conn.Close()
	if err != nil {
		return
	}
	time.Sleep(50 * time.Millisecond)
}
