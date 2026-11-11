package tunnel

import (
	"context"
	"errors"
	"net"
)

// Handler 接管专用客户端，仅供一次拨号；成功后资源交给返回的连接。
// Close 释放尚未交出的客户端，可在调用 DialContext 的作用域中 defer。
type Handler interface {
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
	Close() error
}

type Option func() (Handler, error)

// New 接管 option 提供的专用资源，构造失败时释放已接收的资源。
func New(option Option) (Handler, error) {
	if option == nil {
		return nil, errors.New("tunnel option is required")
	}
	handler, err := option()
	if err != nil {
		if handler != nil {
			_ = handler.Close()
		}
		return nil, err
	}
	if handler == nil {
		return nil, errors.New("tunnel handler is required")
	}
	return handler, nil
}
