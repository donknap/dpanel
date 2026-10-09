package docker

import (
	"context"
	"crypto/tls"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	dockerclient "github.com/docker/docker/client"
	"github.com/docker/go-connections/tlsconfig"
	dockerconn "github.com/donknap/dpanel/common/service/docker/conn"
	"github.com/donknap/dpanel/common/service/docker/conn/listener"
	"github.com/donknap/dpanel/common/service/docker/types"
	"github.com/donknap/dpanel/common/service/ssh"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/gin-gonic/gin"
)

var Sdk *Client

func NewClientWithUser(http *gin.Context) (*Client, error) {
	// TODO 用户级 Docker Client 尚未落地，当前仍返回全局 Sdk。HTTP 请求链路允许分阶段迁移，
	// 未改造的调用点继续使用 docker.Sdk，待共享连接 Map 实现后统一收口。
	if Sdk == nil || Sdk.Client == nil || Sdk.DockerEnv == nil {
		return nil, errors.New("docker client is not initialized")
	}
	return Sdk, nil
}

func NewClientWithDockerEnv(dockerEnv *types.DockerEnv, opts ...Option) (*Client, error) {
	options := make([]Option, 0)
	options = append(options, WithSockName(dockerEnv.GetSockName()))
	options = append(options, WithDockerEnv(dockerEnv))
	options = append(options, WithName(dockerEnv.Name))
	switch dockerEnv.RemoteType {
	case define.DockerRemoteTypeSSH:
		options = append(options, WithSSH(dockerEnv.SshServerInfo, define.DockerConnectServerTimeout))
	case define.DockerRemoteTypeWSL:
		options = append(options, WithWSL(dockerEnv.Address))
	case define.DockerRemoteTypeTcp:
		options = append(options, WithTCP(dockerEnv.Address, dockerEnv.EnableTLS, dockerEnv.TlsCa, dockerEnv.TlsCert, dockerEnv.TlsKey))
	case define.DockerRemoteTypeSock:
		options = append(options, WithSock(dockerEnv.Address))
	default:
		return nil, errors.New("invalid Docker remote type")
	}
	options = append(options, opts...)
	return NewClient(options...)
}

func NewEmptyClient(dockerEnv *types.DockerEnv) *Client {
	v, err := NewClient(
		WithSock(dockerclient.DefaultDockerHost),
		WithName(dockerEnv.Name),
		WithDockerEnv(dockerEnv),
	)
	if err != nil {
		panic(err)
	}
	return v
}

func NewClient(opts ...Option) (*Client, error) {
	c := &Client{
		Name: define.DockerDefaultClientName,
		Option: []dockerclient.Opt{
			dockerclient.FromEnv,
			dockerclient.WithAPIVersionNegotiation(),
			dockerclient.WithUserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:151.0) Gecko/20100101 Firefox/151.0"),
		},
		Client: &dockerclient.Client{},
	}

	if c.Ctx == nil {
		c.Ctx, c.CtxCancelFunc = context.WithCancel(context.Background())
	}

	for _, opt := range opts {
		err := opt(c)
		if err != nil {
			return nil, err
		}
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
	Host          string
	Client        *dockerclient.Client
	Option        []dockerclient.Opt
	Ctx           context.Context
	CtxCancelFunc context.CancelFunc
	DockerEnv     *types.DockerEnv
	proxyListener net.Listener
	proxyServer   *http.Server
	proxySockName string
}

func (self *Client) Close() {
	if self.proxyServer != nil {
		_ = self.proxyServer.Close()
	}
	if self.proxyListener != nil {
		_ = self.proxyListener.Close()
	}
	if self.CtxCancelFunc != nil {
		self.CtxCancelFunc()
	}
	if self.Client != nil {
		_ = self.Client.Close()
	}
}

// GetTryCtx 获取一个有超时的上下文，用于测试 docker 连接是否正常
func (self *Client) GetTryCtx() context.Context {
	timeout := define.DockerConnectServerTimeout
	// 本机 sock 连接可能启动较慢，不限制探测超时。
	if !self.DockerEnv.IsRemote() {
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

// WithSockName 指定 WithSockProxy 使用的本地代理入口，不改变 Docker SDK 的上游地址。
func WithSockName(name string) Option {
	return func(self *Client) error {
		self.proxySockName = name
		return nil
	}
}

func WithSock(host string) Option {
	return func(self *Client) error {
		endpoint, err := url.Parse(host)
		if err != nil {
			return err
		}
		if endpoint.Scheme != "unix" && endpoint.Scheme != "npipe" {
			return errors.New("invalid Docker socket address")
		}
		self.Option = append(self.Option, dockerclient.WithHost(host))
		self.Host = host
		return nil
	}
}

func WithDockerEnv(info *types.DockerEnv) Option {
	return func(self *Client) error {
		if info.DockerStatus == nil {
			info.DockerStatus = &types.DockerStatus{
				Available: false,
				Message:   "",
			}
		}
		self.DockerEnv = info
		return nil
	}
}

func WithTCP(address string, enableTLS bool, caPath, certPath, keyPath string) Option {
	return func(self *Client) error {
		endpoint, err := url.Parse(address)
		if err != nil {
			return err
		}
		if endpoint.Scheme != "tcp" || endpoint.Host == "" {
			return errors.New("invalid TCP Docker address")
		}
		var config *tls.Config
		if enableTLS {
			if caPath == "" || certPath == "" || keyPath == "" {
				return errors.New("invalid TLS configuration")
			}
			certRealPath := map[string]string{
				"ca":   filepath.Join(storage.Local{}.GetCertPath(), caPath),
				"cert": filepath.Join(storage.Local{}.GetCertPath(), certPath),
				"key":  filepath.Join(storage.Local{}.GetCertPath(), keyPath),
			}
			for _, path := range certRealPath {
				if _, err := os.Stat(path); err != nil {
					return errors.New("cert file not found: " + path)
				}
			}
			config, err = tlsconfig.Client(tlsconfig.Options{
				CAFile:             certRealPath["ca"],
				CertFile:           certRealPath["cert"],
				KeyFile:            certRealPath["key"],
				ExclusiveRootPools: true,
			})
			if err != nil {
				return err
			}
		}
		dialContext := func(ctx context.Context, _, _ string) (net.Conn, error) {
			return dockerconn.NewTCP(ctx, endpoint.Host, config)
		}
		transport := &http.Transport{
			IdleConnTimeout:     time.Minute,
			MaxIdleConnsPerHost: 5,
			MaxIdleConns:        100,
			DialContext:         dialContext,
		}
		if enableTLS {
			// SDK 使用 https，代理使用 http；两者都复用 TCP conn 已完成的 TLS 握手。
			transport.DialTLSContext = dialContext
			self.Option = append(self.Option, dockerclient.WithScheme("https"))
		}
		self.Option = append(self.Option, dockerclient.WithHost(address), dockerclient.WithHTTPClient(&http.Client{Transport: transport}))
		self.Host = address
		return nil
	}
}

func WithSSH(serverInfo *ssh.ServerInfo, timeout time.Duration) Option {
	return func(self *Client) error {
		cmdName := "docker"
		if self.DockerEnv.DockerType == "podman" {
			cmdName = "podman"
		}
		lock := sync.Mutex{}
		transport := &http.Transport{
			// 放开长连接复用，提升并发性能
			DisableKeepAlives: false,
			// 设置空闲回收时间。如果一个 SSH 连接 1 分钟没请求，自动回收
			IdleConnTimeout: 1 * time.Minute,
			// 限制针对该宿主机的最大闲置连接数
			// 确保在高并发后，池子里最多只留几个连接备用，多余的会立即物理断开
			MaxIdleConnsPerHost: 5,
			// 限制总闲置连接数
			MaxIdleConns: 100,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				lock.Lock()
				if serverInfo == nil {
					lock.Unlock()
					slog.Warn("ssh client nil", "name", self.Name, "dockerEnv", self.DockerEnv)
					return nil, errors.New("nil serverInfo")
				}
				opts := ssh.WithServerInfo(serverInfo)
				opts = append(opts, ssh.WithConnectContext(ctx))
				opts = append(opts, ssh.WithTimeout(timeout))
				sshClient, err := ssh.NewClient(opts...)
				lock.Unlock()
				if err != nil {
					return nil, err
				}

				// 直接返回包装好的 Conn，完全由 http.Client 的生命周期来控制底层 SSH Client 的闭合，去掉了之前导致泄漏的监听协程
				conn, err := dockerconn.NewSSH(sshClient, cmdName, "system", "dial-stdio")
				if err != nil {
					sshClient.Close()
					return nil, err
				}
				return conn, nil
			},
		}
		self.Option = append(self.Option, dockerclient.WithHTTPClient(&http.Client{Transport: transport}))
		return nil
	}
}

func WithWSL(address string) Option {
	return func(self *Client) error {
		if runtime.GOOS != "windows" {
			return errors.New("WSL Docker connection requires Windows")
		}
		if !strings.HasPrefix(address, "wsl://") {
			return errors.New("invalid WSL Docker address")
		}
		distribution := strings.TrimPrefix(address, "wsl://")
		if distribution == "" || strings.ContainsAny(distribution, "\\/\r\n\x00") {
			return errors.New("invalid WSL distribution")
		}
		transport := &http.Transport{
			IdleConnTimeout:     time.Minute,
			MaxIdleConnsPerHost: 5,
			MaxIdleConns:        100,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				return dockerconn.NewWSL(distribution, "docker")
			},
		}
		self.Option = append(self.Option, dockerclient.WithHTTPClient(&http.Client{Transport: transport}))
		return nil
	}
}

// WithSockProxy 作为 Option 在 Docker SDK Client 构造前执行，上游拨号要等请求到达后才能取得 Client。
type lazyProxyTransport struct {
	client *Client
}

func (t *lazyProxyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.client == nil || t.client.Client == nil {
		return nil, errors.New("Docker client is not initialized")
	}
	httpClient := t.client.Client.HTTPClient()
	if httpClient == nil || httpClient.Transport == nil {
		return nil, errors.New("Docker client is not initialized")
	}
	return httpClient.Transport.RoundTrip(req)
}

func WithSockProxy() Option {
	return func(self *Client) error {
		if self.DockerEnv.RemoteType == define.DockerRemoteTypeSock {
			return nil
		}
		address := self.proxySockName
		localSock, _, err := listener.New(address)
		slog.Debug("docker with socket proxy", "address", address, "sock", localSock)
		if err != nil {
			return err
		}
		self.proxyListener = localSock

		go func() {
			<-self.Ctx.Done()
			_ = localSock.Close()
		}()

		proxy := &httputil.ReverseProxy{
			Rewrite: func(req *httputil.ProxyRequest) {
				req.Out.URL.Scheme = "http"
				req.Out.URL.Host = "api.dpanel.localhost"
				req.Out.RequestURI = ""
			},
			Transport: &lazyProxyTransport{client: self},
			ModifyResponse: func(r *http.Response) error {
				return nil
			},
			ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
				if !errors.Is(err, context.Canceled) {
					slog.Warn("proxy error", "err", err)
				}
			},
		}

		self.proxyServer = &http.Server{Handler: proxy}
		go func() {
			err := self.proxyServer.Serve(localSock)
			// 过滤掉因为正常关闭而产生的日志噪音
			if err != nil && err != http.ErrServerClosed && !strings.Contains(err.Error(), "use of closed network connection") {
				slog.Warn("local sock proxy exited", "err", err)
			}
		}()

		return nil
	}
}
