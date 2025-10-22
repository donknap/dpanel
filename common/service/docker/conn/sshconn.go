// Package sshconn provides a net.Conn implementation that connects to a remote
// Docker daemon via SSH by running "docker system dial-stdio".
//
// Example:
//
//	httpClient := &http.Client{
//		Transport: &http.Transport{
//			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
//				return sshconn.New(ctx, sshClient, "docker", "system", "dial-stdio")
//			},
//		},
//	}
package sshconn

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/donknap/dpanel/common/service/ssh"
	ssh2 "golang.org/x/crypto/ssh"
)

// New returns a net.Conn that runs the given command via SSH.
// The command should provide a stdio-based protocol (e.g., "docker system dial-stdio").
func New(sshClient *ssh.Client, cmd string, args ...string) (net.Conn, error) {
	// Do not cancel the SSH session when ctx is cancelled.
	// The lifetime should be managed by the http.Client, not the dial context.
	session, err := sshClient.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create ssh session: %w", err)
	}

	// Build full command for logging
	fullCmd := cmd
	if len(args) > 0 {
		fullCmd = cmd + " " + strings.Join(args, " ")
	}

	// Setup pipes
	stdin, err := session.StdinPipe()
	if err != nil {
		_ = session.Close()
		return nil, fmt.Errorf("failed to get stdin pipe: %w", err)
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		_ = session.Close()
		return nil, fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	// Capture stderr with writer
	c := &sshConn{
		session:    session,
		stdin:      stdin,
		stdout:     stdout,
		localAddr:  dummyAddr{network: "ssh", s: "local"},
		remoteAddr: dummyAddr{network: "ssh", s: "remote"},
		fullCmd:    fullCmd,
	}
	session.Stderr = &stderrWriter{
		stderrMu:    &c.stderrMu,
		stderr:      &c.stderr,
		debugPrefix: fmt.Sprintf("sshconn (%s):", fullCmd),
	}

	// Start the command
	if err := session.Start(fullCmd); err != nil {
		_ = session.Close()
		return nil, fmt.Errorf("failed to start command %q: %w", fullCmd, err)
	}
	go func() {
		<-sshClient.Ctx().Done()
		c.kill()
	}()

	slog.Debug("sshconn: started", "cmd", fullCmd)
	return c, nil
}

// sshConn implements net.Conn
type sshConn struct {
	session *ssh2.Session

	cmdMutex   sync.Mutex // Protects session.Wait() and cmdWaitErr
	cmdWaitErr error
	cmdExited  atomic.Bool // true after session.Wait() returns

	stdin    io.WriteCloser
	stdout   io.Reader
	stderrMu sync.Mutex // Protects stderr buffer
	stderr   bytes.Buffer

	localAddr  net.Addr
	remoteAddr net.Addr
	fullCmd    string // for logging
}

// kill terminates the SSH session gracefully
func (c *sshConn) kill() {
	if c.cmdExited.Load() {
		return
	}

	c.cmdMutex.Lock()
	defer c.cmdMutex.Unlock()

	if c.cmdExited.Load() {
		return
	}

	done := make(chan error, 1)
	go func() { done <- c.session.Wait() }()

	var werr error
	select {
	case werr = <-done:
	case <-time.After(3 * time.Second):
		_ = c.session.Close() // force close
		werr = <-done
	}

	c.cmdWaitErr = werr
	c.cmdExited.Store(true)
}

// handleEOF handles io.EOF from Read/Write
func (c *sshConn) handleEOF(err error) error {
	if err != io.EOF {
		return err
	}

	c.cmdMutex.Lock()
	defer c.cmdMutex.Unlock()

	var werr error
	if c.cmdExited.Load() {
		werr = c.cmdWaitErr
	} else {
		done := make(chan error, 1)
		go func() { done <- c.session.Wait() }()
		select {
		case werr = <-done:
			c.cmdWaitErr = werr
			c.cmdExited.Store(true)
		case <-time.After(10 * time.Second):
			c.stderrMu.Lock()
			stderr := c.stderr.String()
			c.stderrMu.Unlock()
			return fmt.Errorf(
				"ssh command %q did not exit after EOF within 10s: stderr=%q",
				c.fullCmd, stderr,
			)
		}
	}

	if werr == nil {
		return err
	}

	c.stderrMu.Lock()
	stderr := c.stderr.String()
	c.stderrMu.Unlock()
	return fmt.Errorf(
		"ssh command %q exited with error: %v, stderr=%q",
		c.fullCmd, werr, stderr,
	)
}

func (c *sshConn) Read(p []byte) (n int, err error) {
	n, err = c.stdout.Read(p)
	return n, c.handleEOF(err)
}

func (c *sshConn) Write(p []byte) (n int, err error) {
	n, err = c.stdin.Write(p)
	return n, c.handleEOF(err)
}

// CloseRead closes the read side (stdout)
func (c *sshConn) CloseRead() error {
	return c.Close()
}

// CloseWrite closes the write side (stdin)
func (c *sshConn) CloseWrite() error {
	return c.Close()
}

// Close implements net.Conn.Close
func (c *sshConn) Close() error {
	_ = c.stdin.Close()

	c.kill()
	return nil
}

func (c *sshConn) LocalAddr() net.Addr {
	return c.localAddr
}

func (c *sshConn) RemoteAddr() net.Addr {
	return c.remoteAddr
}

func (*sshConn) SetDeadline(t time.Time) error {
	slog.Debug("unimplemented call: SetDeadline", "time", t)
	return nil
}

func (*sshConn) SetReadDeadline(t time.Time) error {
	slog.Debug("unimplemented call: SetReadDeadline", "time", t)
	return nil
}

func (*sshConn) SetWriteDeadline(t time.Time) error {
	slog.Debug("unimplemented call: SetWriteDeadline", "time", t)
	return nil
}

// dummyAddr implements net.Addr
type dummyAddr struct {
	network string
	s       string
}

func (d dummyAddr) Network() string { return d.network }
func (d dummyAddr) String() string  { return d.s }

// stderrWriter captures stderr and logs it
type stderrWriter struct {
	stderrMu    *sync.Mutex
	stderr      *bytes.Buffer
	debugPrefix string
}

func (w *stderrWriter) Write(p []byte) (int, error) {
	// Log every line
	slog.Debug("debug write", "prefix", w.debugPrefix, "error", string(p))

	w.stderrMu.Lock()
	defer w.stderrMu.Unlock()

	// Limit buffer size to prevent memory leak
	if w.stderr.Len() > 4096 {
		w.stderr.Reset()
	}
	return w.stderr.Write(p)
}
