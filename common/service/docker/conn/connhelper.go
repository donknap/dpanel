package sshconn

import (
	"bufio"
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"

	"github.com/donknap/dpanel/common/service/ssh"
)

func NewConnection(ctx context.Context, serverInfo *ssh.ServerInfo, listener net.Listener) {
	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()

	opts := ssh.WithServerInfo(serverInfo)
	opts = append(opts, ssh.WithContext(ctx))

	go func() {
		for {
			localConn, err := listener.Accept()
			if err != nil {
				slog.Debug("ssh conn local close", "err", err)
				return
			}
			sshClient, err := ssh.NewClient(opts...)
			if err != nil {
				return
			}
			sshConnection, err := New(sshClient, "docker", "system", "dial-stdio")
			if err != nil {
				slog.Debug("ssh conn create remote", "err", err)
				sshClient.Close()
				return
			}
			go handleProxySession(localConn, sshConnection, sshClient)
		}
	}()
}

// handleProxySession 处理一次完整的代理会话
func handleProxySession(localConn net.Conn, sshConn io.ReadWriteCloser, sshClient *ssh.Client) {
	defer func() {
		_ = localConn.Close()
		_ = sshConn.Close()
		sshClient.Close()
	}()

	req, err := http.ReadRequest(bufio.NewReader(localConn))
	if err != nil {
		slog.Debug("ssh conn handle proxy", "err", err)
		return
	}
	err = req.Write(sshConn)
	if err != nil {
		slog.Debug("ssh conn handle proxy request", "err", err)
		return
	}
	resp, err := http.ReadResponse(bufio.NewReader(sshConn), req)
	if err != nil {
		slog.Debug("ssh conn handle proxy response", "err", err)
		return
	}
	defer resp.Body.Close()

	err = resp.Write(localConn)
	if err != nil {
		slog.Debug("ssh conn handle proxy write sock", "err", err)
		return
	}
}
