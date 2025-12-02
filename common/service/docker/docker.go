package docker

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	dockerclient "github.com/docker/docker/client"
	sshconn "github.com/donknap/dpanel/common/service/docker/conn"
	"github.com/donknap/dpanel/common/service/docker/conn/listener"
	"github.com/donknap/dpanel/common/service/docker/types"
	"github.com/donknap/dpanel/common/service/exec/remote"
	"github.com/donknap/dpanel/common/service/ssh"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/define"
)

var (
	Sdk = NewDefaultClient()
)

func NewClientWithDockerEnv(dockerEnv *types.DockerEnv, opts ...Option) (*Client, error) {
	options := make([]Option, 0)
	options = append(options, WithDockerEnv(dockerEnv))
	options = append(options, WithName(dockerEnv.Name))
	if dockerEnv.EnableTLS {
		options = append(options, WithTLS(dockerEnv.TlsCa, dockerEnv.TlsCert, dockerEnv.TlsKey))
	}
	if dockerEnv.RemoteType == define.DockerRemoteTypeSSH {
		options = append(options, WithSSH(dockerEnv.SshServerInfo, define.DockerConnectServerTimeout))
	} else {
		options = append(options, WithAddress(dockerEnv.Address))
	}
	options = append(options, opts...)
	return NewClient(options...)
}

func NewDefaultClient() *Client {
	defaultDockerHost := dockerclient.DefaultDockerHost
	if e := os.Getenv(dockerclient.EnvOverrideHost); e != "" {
		defaultDockerHost = e
	}
	v, _ := NewClient(WithAddress(defaultDockerHost), WithDockerEnv(&types.DockerEnv{
		Name:    define.DockerDefaultClientName,
		Title:   define.DockerDefaultClientName,
		Address: defaultDockerHost,
		Default: true,
	}))
	return v
}

func NewClient(opts ...Option) (*Client, error) {
	c := &Client{
		Name: define.DockerDefaultClientName,
		Option: []dockerclient.Opt{
			dockerclient.FromEnv,
			dockerclient.WithAPIVersionNegotiation(),
		},
	}

	for _, opt := range opts {
		err := opt(c)
		if err != nil {
			return nil, err
		}
	}

	if c.Ctx == nil {
		c.Ctx, c.CtxCancelFunc = context.WithCancel(context.Background())
	}

	obj, err := dockerclient.NewClientWithOpts(c.Option...)
	if err != nil {
		c.Close()
		return nil, err
	}
	c.Client = obj
	return c, nil
}

type Client struct {
	Name          string
	Client        *dockerclient.Client
	Option        []dockerclient.Opt
	Ctx           context.Context
	CtxCancelFunc context.CancelFunc
	DockerEnv     *types.DockerEnv
}

func (self Client) Close() {
	if self.DockerEnv.RemoteType == define.DockerRemoteTypeSSH {
		localProxySock := filepath.Join(storage.Local{}.GetLocalProxySockPath(), fmt.Sprintf("%s.sock", self.Name))
		if strings.Contains(self.Client.DaemonHost(), self.Client.DaemonHost()) {
			_ = os.Remove(localProxySock)
		}
	}
	if self.CtxCancelFunc != nil {
		self.CtxCancelFunc()
	}
	if self.Client != nil {
		_ = self.Client.Close()
	}
}

// GetTryCtx 获取一个有超时的上下文，用于测试 docker 连接是否正常
func (self Client) GetTryCtx() context.Context {
	timeout := define.DockerConnectServerTimeout
	// 如果使用 docker.sock 则不要超时时间，某些系统可能启动慢
	if strings.HasSuffix(self.DockerEnv.Address, "docker.sock") {
		return self.Ctx
	}
	tryCtx, _ := context.WithTimeout(context.Background(), timeout)
	return tryCtx
}

type Option func(builder *Client) error

func WithName(name string) Option {
	return func(self *Client) error {
		self.Name = name
		return nil
	}
}

func WithAddress(host string) Option {
	return func(self *Client) error {
		self.Option = append(self.Option, dockerclient.WithHost(host))
		return nil
	}
}

func WithDockerEnv(info *types.DockerEnv) Option {
	return func(self *Client) error {
		self.DockerEnv = info
		return nil
	}
}

func WithTLS(caPath, certPath, keyPath string) Option {
	certRealPath := map[string]string{
		"ca":   filepath.Join(storage.Local{}.GetCertPath(), caPath),
		"cert": filepath.Join(storage.Local{}.GetCertPath(), certPath),
		"key":  filepath.Join(storage.Local{}.GetCertPath(), keyPath),
	}
	return func(self *Client) error {
		if caPath == "" || certPath == "" || keyPath == "" {
			return errors.New("invalid TLS configuration")
		}
		for _, path := range certRealPath {
			if _, err := os.Stat(path); err != nil {
				return errors.New("cert file not found: " + path)
			}
		}

		self.Option = append(self.Option, dockerclient.WithTLSClientConfig(
			certRealPath["ca"],
			certRealPath["cert"],
			certRealPath["key"],
		))
		return nil
	}
}

func WithSSH(serverInfo *ssh.ServerInfo, timeout time.Duration) Option {
	return func(self *Client) error {
		cmdName := "docker"
		if sshClient, err := ssh.NewClient(ssh.WithServerInfo(serverInfo)...); err == nil {
			if content, err := remote.QuickRun(sshClient, "podman version"); err == nil && strings.Contains(string(content), "Podman Engine") {
				slog.Debug("docker with ssh podman", "version", string(content))
				cmdName = "podman"
			}
		} else {
			return err
		}
		lock := sync.Mutex{}
		transport := &http.Transport{
			DisableKeepAlives: false,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				lock.Lock()
				opts := ssh.WithServerInfo(serverInfo)
				opts = append(opts, ssh.WithContext(ctx))
				opts = append(opts, ssh.WithTimeout(timeout))
				sshClient, err := ssh.NewClient(opts...)
				lock.Unlock()
				if err != nil {
					return nil, err
				}
				return sshconn.New(sshClient, cmdName, "system", "dial-stdio")
			},
		}
		self.Option = append(self.Option, dockerclient.WithHTTPClient(&http.Client{Transport: transport}))
		time.AfterFunc(time.Second, func() {
			// 这里稍微延迟一下，防止 ssh 还没有连接完成
			err := WithSockProxy()(self)
			if err != nil {
				slog.Debug("local sock proxy", "err", err)
			}
		})
		return nil
	}
}

func WithSockProxy() Option {
	return func(self *Client) error {
		// 创建代理 sock
		sockPath := ""
		if runtime.GOOS == "windows" {
			sockPath = self.Name
		} else {
			localProxySock := filepath.Join(storage.Local{}.GetLocalProxySockPath(), fmt.Sprintf("%s.sock", self.Name))
			_ = os.Remove(localProxySock)
			sockPath = localProxySock
		}
		localSock, _, err := listener.New(sockPath)
		if err != nil {
			return err
		}

		go func() {
			<-self.Ctx.Done()
			_ = localSock.Close()
		}()

		go func() {
			for {
				localConn, err := localSock.Accept()
				if err != nil {
					slog.Debug("local sock", "err", err)
					return
				}
				go func() {
					err = func() error {
						defer func() {
							err = localConn.Close()
							if err != nil {
								slog.Debug("local conn close", err)
							}
						}()
						req, err := http.ReadRequest(bufio.NewReader(localConn))
						if err != nil {
							return err
						}
						slog.Debug("local conn request", "url", req.URL, "host", req.Host)
						defer func() {
							_ = req.Body.Close()
						}()
						req.URL.Scheme = "http"
						req.URL.Host = "api.dpanel.localhost"
						req.RequestURI = ""
						resp, err := self.Client.HTTPClient().Do(req)
						if err != nil {
							return err
						}
						defer func() {
							_ = resp.Body.Close()
						}()
						err = resp.Write(localConn)
						if err != nil {
							return err
						}
						return nil
					}()
					if err != nil {
						slog.Debug("local sock", "err", err)
					}
				}()
			}
		}()
		return nil
	}
}
