package sshconn

import (
	"context"
	"io"
	"log/slog"
	"net"

	"github.com/donknap/dpanel/common/service/ssh"
)

func NewConnection(ctx context.Context, sshClient *ssh.Client, listener net.Listener) {
	go func() {
		select {
		case <-ctx.Done():
			_ = listener.Close()
		}
	}()

	go func() {
		for {
			localConn, err := listener.Accept()
			if err != nil {
				slog.Warn("docker proxy sock local close", "err", err)
				return
			}

			sshConn, err := New(sshClient, "docker", "system", "dial-stdio")
			if err != nil {
				slog.Warn("docker proxy sock create remote", "err", err)
				return
			}

			go func() {
				_, _ = io.Copy(sshConn, localConn)
				_ = sshConn.Close()
			}()
			go func() {
				_, _ = io.Copy(localConn, sshConn)
				_ = localConn.Close()
			}()
		}
	}()
}
