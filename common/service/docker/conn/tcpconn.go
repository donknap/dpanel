package conn

import (
	"context"
	"crypto/tls"
	"net"
)

// NewTCP opens a Docker TCP connection, negotiating TLS when configured.
func NewTCP(ctx context.Context, address string, config *tls.Config) (net.Conn, error) {
	dialer := &net.Dialer{}
	if config != nil {
		return (&tls.Dialer{NetDialer: dialer, Config: config}).DialContext(ctx, "tcp", address)
	}
	return dialer.DialContext(ctx, "tcp", address)
}
