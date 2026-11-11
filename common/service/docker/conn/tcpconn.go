package conn

import (
	"context"
	"crypto/tls"
	"errors"
	"net"

	"github.com/docker/go-connections/tlsconfig"
)

type TCPOption struct {
	EnableTLS bool
	CaPath    string
	CertPath  string
	KeyPath   string
	Dial      func(context.Context, string, string) (net.Conn, error) // nil 使用直连。
}

// NewTCP 返回可直接收发数据的连接；TLS 握手失败时关闭底层连接。
func NewTCP(ctx context.Context, address string, option TCPOption) (net.Conn, error) {
	var config *tls.Config
	if option.EnableTLS {
		if option.CaPath == "" || option.CertPath == "" || option.KeyPath == "" {
			return nil, errors.New("invalid TLS configuration")
		}
		var err error
		config, err = tlsconfig.Client(tlsconfig.Options{
			CAFile:             option.CaPath,
			CertFile:           option.CertPath,
			KeyFile:            option.KeyPath,
			ExclusiveRootPools: true,
		})
		if err != nil {
			return nil, err
		}
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		// 证书按目标主机校验，不能使用转发连接的远端地址。
		config.ServerName = host
	}
	dialContext := option.Dial
	if dialContext == nil {
		dialContext = (&net.Dialer{}).DialContext
	}
	conn, err := dialContext(ctx, "tcp", address)
	if err != nil {
		return nil, err
	}
	if config == nil {
		return conn, nil
	}
	tlsConn := tls.Client(conn, config)
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return tlsConn, nil
}
