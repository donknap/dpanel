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

func NewConnection(ctx context.Context, opts []ssh.Option, listener net.Listener) {
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

			sshClient, err := ssh.NewClient(opts...)
			if err != nil {
				return
			}
			sshConnection, err := New(sshClient, "docker", "system", "dial-stdio")
			if err != nil {
				sshClient.Close()
				slog.Warn("docker proxy sock create remote", "err", err)
				return
			}

			go handleProxySession(localConn, sshConnection, sshClient)
		}
	}()
}

// handleProxySession 处理一次完整的代理会话
func handleProxySession(localConn net.Conn, sshConn io.ReadWriteCloser, sshClient *ssh.Client) {
	defer localConn.Close()
	defer sshConn.Close()
	defer sshClient.Close()

	// 1. 读取 HTTP 请求
	bufr := bufio.NewReader(localConn)
	req, err := http.ReadRequest(bufr)
	if err != nil {
		slog.Debug("read request failed", "err", err)
		return
	}

	// 2. 修改请求头：添加 Connection: close
	req.Header.Set("Connection", "close") // ⚠️ 关键：让 dockerd 主动关闭连接
	// 可选：删除可能引起问题的头
	req.RequestURI = "" // 标准要求
	if req.Host == "" {
		req.Host = "localhost" // 避免某些情况下的错误
	}

	// 3. 将修改后的请求写入 SSH 连接
	err = req.Write(sshConn)
	if err != nil {
		slog.Debug("write request to ssh failed", "err", err)
		return
	}

	// 4. 读取 HTTP 响应
	resp, err := http.ReadResponse(bufio.NewReader(sshConn), req)
	if err != nil {
		slog.Debug("read response failed", "err", err)
		return
	}
	defer resp.Body.Close()

	// 5. 将响应写回本地连接
	err = resp.Write(localConn)
	if err != nil {
		slog.Debug("write response to local failed", "err", err)
		return
	}

	// 6. ✅ 响应已完整写出，立即关闭所有连接
	slog.Debug("Response sent, closing connections immediately")
	// defer 会自动关闭
}
