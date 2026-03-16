package docker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
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
	"github.com/donknap/dpanel/common/service/ssh"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/define"
)

var Sdk *Client

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

func NewEmptyClient(dockerEnv *types.DockerEnv) *Client {
	v, err := NewClient(
		WithAddress(define.DockerDefaultEmptyClientHost),
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
}

func (self *Client) Close() {
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
		self.Host = host
		return nil
	}
}

func WithDockerEnv(info *types.DockerEnv) Option {
	return func(self *Client) error {
		if info != nil && strings.HasPrefix(info.Address, "tcp://") && info.RemoteType != define.DockerRemoteTypeSSH {
			info.RemoteType = define.DockerRemoteTypeTcp
		}
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
					slog.Debug("ssh client nil", "name", self.Name, "dockerEnv", self.DockerEnv)
					return nil, errors.New("nil serverInfo")
				}
				opts := ssh.WithServerInfo(serverInfo)
				opts = append(opts, ssh.WithContext(ctx))
				opts = append(opts, ssh.WithTimeout(timeout))
				sshClient, err := ssh.NewClient(opts...)
				lock.Unlock()
				if err != nil {
					return nil, err
				}

				// 直接返回包装好的 Conn，完全由 http.Client 的生命周期来控制底层 SSH Client 的闭合，去掉了之前导致泄漏的监听协程
				conn, err := sshconn.New(sshClient, cmdName, "system", "dial-stdio")
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

// 【新增结构体】：延迟加载底层的 Transport
// 避免在 Client 尚未完全构建完成时发生 HTTPClient() 的空指针异常
type lazyProxyTransport struct {
	client *Client
}

func (t *lazyProxyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// 在请求真正发生时，Docker Client 一定已经初始化完毕
	if t.client != nil && t.client.Client != nil {
		hc := t.client.Client.HTTPClient()
		if hc != nil && hc.Transport != nil {
			return hc.Transport.RoundTrip(req)
		}
	}
	return http.DefaultTransport.RoundTrip(req)
}

func WithSockProxy() Option {
	return func(self *Client) error {
		if self.DockerEnv.RemoteType != define.DockerRemoteTypeSSH {
			return nil
		}
		// 这里稍微延迟一下，防止 ssh 还没有连接完成
		time.Sleep(time.Second * 2)
		// 创建代理 sock
		sockPath := ""
		if runtime.GOOS == "windows" {
			sockPath = self.Name
		} else {
			localProxySock := filepath.Join(storage.Local{}.GetLocalProxySockPath(), fmt.Sprintf("%s.sock", self.Name))
			slog.Debug("local sock path remove", "path", localProxySock)
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

		// 【关键优化】：使用标准库 httputil.ReverseProxy 替代原先手写的残缺版代理逻辑
		proxy := &httputil.ReverseProxy{
			Director: func(req *http.Request) {
				req.URL.Scheme = "http"
				req.URL.Host = "api.dpanel.localhost"
				// 必须清空 RequestURI，否则 http.Client 拨号会报错
				req.RequestURI = ""
			},
			// 【修复点 2】：利用上面定义的延迟加载器，代替直接读取 self.Client.HTTPClient().Transport
			Transport: &lazyProxyTransport{client: self},
			ModifyResponse: func(r *http.Response) error {
				return nil
			},
			ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
				if err != context.Canceled {
					slog.Debug("proxy error", "err", err)
				}
			},
		}

		go func() {
			server := &http.Server{
				Handler: proxy,
			}
			err := server.Serve(localSock)
			// 过滤掉因为正常关闭而产生的日志噪音
			if err != nil && err != http.ErrServerClosed && !strings.Contains(err.Error(), "use of closed network connection") {
				slog.Debug("local sock proxy exited", "err", err)
			}
		}()

		return nil
	}
}
