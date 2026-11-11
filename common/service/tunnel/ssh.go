package tunnel

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/donknap/dpanel/common/service/ssh"
)

// WithSSH 将已连接的专用客户端交给 New 接管，不适用于共享客户端。
func WithSSH(client *ssh.Client) Option {
	return func() (Handler, error) {
		handler := &sshTunnel{client: client}
		if client == nil || client.Conn == nil {
			return handler, errors.New("SSH tunnel requires a connected client")
		}
		return handler, nil
	}
}

type sshTunnel struct {
	mu     sync.Mutex
	client *ssh.Client
}

func (self *sshTunnel) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	self.mu.Lock()
	client := self.client
	self.client = nil
	self.mu.Unlock()
	if client == nil {
		return nil, errors.New("SSH tunnel client is no longer available")
	}
	if network != "tcp" && network != "tcp4" && network != "tcp6" {
		client.Close()
		return nil, fmt.Errorf("unsupported SSH tunnel network: %s", network)
	}
	connectCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	conn, err := client.Conn.DialContext(connectCtx, network, address)
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("open SSH tunnel to %s: %w", address, err)
	}
	slog.Info("SSH tunnel connected", "jump", client.Conn.RemoteAddr().String(), "target", address)
	return &sshConn{Conn: conn, client: client}, nil
}

func (self *sshTunnel) Close() error {
	self.mu.Lock()
	client := self.client
	self.client = nil
	self.mu.Unlock()
	if client != nil {
		client.Close()
	}
	return nil
}

type sshConn struct {
	net.Conn
	client *ssh.Client
}

func (self *sshConn) CloseWrite() error {
	conn, ok := self.Conn.(interface{ CloseWrite() error })
	if !ok {
		return errors.New("SSH tunnel connection does not support CloseWrite")
	}
	return conn.CloseWrite()
}

func (self *sshConn) Close() error {
	err := self.Conn.Close()
	self.client.Close()
	return err
}
