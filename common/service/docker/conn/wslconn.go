package conn

import (
	"fmt"
	"io"
	"net"
	"os/exec"
	"sync"
	"time"
)

// NewWSL starts a Docker stdio connection in the selected WSL distribution.
func NewWSL(distribution, command string) (net.Conn, error) {
	cmd := exec.Command("wsl.exe", "-d", distribution, "--", command, "system", "dial-stdio")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("open WSL stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("open WSL stdout: %w", err)
	}
	cmd.Stderr = io.Discard
	if err = cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, fmt.Errorf("start WSL Docker connection: %w", err)
	}
	return &wslConn{cmd: cmd, stdin: stdin, stdout: stdout}, nil
}

type wslConn struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	close  sync.Once
}

func (c *wslConn) Read(p []byte) (int, error)  { return c.stdout.Read(p) }
func (c *wslConn) Write(p []byte) (int, error) { return c.stdin.Write(p) }

func (c *wslConn) Close() error {
	c.close.Do(func() {
		_ = c.stdin.Close()
		_ = c.stdout.Close()
		if c.cmd.Process != nil {
			_ = c.cmd.Process.Kill()
		}
		_ = c.cmd.Wait()
	})
	return nil
}

func (c *wslConn) LocalAddr() net.Addr              { return dummyAddr{network: "wsl", s: "local"} }
func (c *wslConn) RemoteAddr() net.Addr             { return dummyAddr{network: "wsl", s: "remote"} }
func (c *wslConn) SetDeadline(time.Time) error      { return nil }
func (c *wslConn) SetReadDeadline(time.Time) error  { return nil }
func (c *wslConn) SetWriteDeadline(time.Time) error { return nil }
