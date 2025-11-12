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

	"github.com/docker/docker/client"
	sshconn "github.com/donknap/dpanel/common/service/docker/conn"
	"github.com/donknap/dpanel/common/service/docker/conn/listener"
	"github.com/donknap/dpanel/common/service/ssh"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
)

var (
	Sdk               = &Builder{}
	BuilderAuthor     = "DPanel"
	BuildDesc         = "DPanel is a lightweight Docker web management panel"
	BuildWebSite      = "https://dpanel.cc"
	BuildVersion      = "1.0.0"
	DefaultClientName = "local"
)

func S() *Builder {
	return Sdk
}

type ClientDockerInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Client struct {
	Name              string            `json:"name,omitempty" binding:"required"`
	Title             string            `json:"title,omitempty" binding:"required"`
	Address           string            `json:"address,omitempty" binding:"required"` // docker api 地址
	Default           bool              `json:"default,omitempty"`                    // 是否是默认客户端
	DockerInfo        *ClientDockerInfo `json:"dockerInfo,omitempty"`
	ServerUrl         string            `json:"serverUrl,omitempty"`
	EnableTLS         bool              `json:"enableTLS,omitempty"`
	TlsCa             string            `json:"tlsCa,omitempty"`
	TlsCert           string            `json:"tlsCert,omitempty"`
	TlsKey            string            `json:"tlsKey,omitempty"`
	EnableComposePath bool              `json:"enableComposePath,omitempty"` // 启用 compose 独享目录
	ComposePath       string            `json:"composePath,omitempty"`
	EnableSSH         bool              `json:"enableSSH,omitempty"`
	SshServerInfo     *ssh.ServerInfo   `json:"sshServerInfo,omitempty"`
	RemoteType        string            `json:"remoteType"` // 远程客户端类型，支持 docker ssh
}

func (self Client) CommandEnv() []string {
	result := make([]string, 0)
	if self.RemoteType == RemoteTypeSSH {
		// 还需要将系统的 PATH 环境变量传递进去，否则可能会报找不到 ssh 命令
		if runtime.GOOS == "windows" {
			result = append(result, fmt.Sprintf("DOCKER_HOST=npipe:////./pipe/dp_%s", self.Name))
		} else {
			result = append(result, fmt.Sprintf("DOCKER_HOST=unix://%s/%s.sock", storage.Local{}.GetLocalProxySockPath(), self.Name))
		}
		result = append(result, os.Environ()...)
		return result
	}
	result = append(result, fmt.Sprintf("DOCKER_HOST=%s", self.Address))
	if self.EnableTLS {
		result = append(result,
			"DOCKER_TLS_VERIFY=1",
			"DOCKER_CERT_PATH="+filepath.Dir(filepath.Join(storage.Local{}.GetCertPath(), self.TlsCa)),
		)
	}
	return result
}

func (self Client) CommandParams() []string {
	result := make([]string, 0)
	if self.RemoteType == RemoteTypeSSH {
		if runtime.GOOS == "windows" {
			result = append(result, "-H", fmt.Sprintf("npipe:////./pipe/dp_%s", self.Name))
		} else {
			result = append(result, "-H", fmt.Sprintf("unix://%s/%s.sock", storage.Local{}.GetLocalProxySockPath(), self.Name))
		}
		return result
	}
	result = append(result, "-H", self.Address)
	if self.EnableTLS {
		result = append(result, "--tlsverify",
			"--tlscacert", filepath.Join(storage.Local{}.GetCertPath(), self.TlsCa),
			"--tlscert", filepath.Join(storage.Local{}.GetCertPath(), self.TlsCert),
			"--tlskey", filepath.Join(storage.Local{}.GetCertPath(), self.TlsKey),
		)
	}
	return result
}

func (self Client) CertRoot() string {
	return filepath.Join("docker", self.Name)
}

type Builder struct {
	Name          string
	Client        *client.Client
	clientOption  []client.Opt
	Ctx           context.Context
	CtxCancelFunc context.CancelFunc
	DockerEnv     *Client
}

func (self Builder) Close() {
	if self.DockerEnv.RemoteType == RemoteTypeSSH {
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
func (self Builder) GetTryCtx() context.Context {
	timeout := define.DockerConnectServerTimeout
	if v := facade.Config.GetDuration("system.docker.init_timeout"); v > 0 {
		timeout = time.Second * v
	}
	tryCtx, _ := context.WithTimeout(context.Background(), timeout)
	return tryCtx
}

func NewBuilderWithDockerEnv(dockerEnv *Client, opts ...Option) (*Builder, error) {
	options := make([]Option, 0)
	options = append(options, WithDockerEnv(dockerEnv))
	options = append(options, WithName(dockerEnv.Name))
	if dockerEnv.EnableTLS {
		options = append(options, WithTLS(dockerEnv.TlsCa, dockerEnv.TlsCert, dockerEnv.TlsKey))
	}
	if dockerEnv.RemoteType == RemoteTypeSSH {
		options = append(options, WithSSH(dockerEnv.SshServerInfo, define.DockerConnectServerTimeout))
	} else {
		options = append(options, WithAddress(dockerEnv.Address))
	}
	options = append(options, opts...)
	return NewBuilder(options...)
}

type Option func(builder *Builder) error

func NewBuilder(opts ...Option) (*Builder, error) {
	c := &Builder{
		Name: "local",
		clientOption: []client.Opt{
			client.FromEnv,
			client.WithAPIVersionNegotiation(),
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

	obj, err := client.NewClientWithOpts(c.clientOption...)
	if err != nil {
		c.Close()
		return nil, err
	}
	c.Client = obj
	return c, nil
}

func WithName(name string) Option {
	return func(self *Builder) error {
		self.Name = name
		return nil
	}
}

func WithAddress(host string) Option {
	return func(self *Builder) error {
		self.clientOption = append(self.clientOption, client.WithHost(host))
		return nil
	}
}

func WithDockerEnv(info *Client) Option {
	return func(self *Builder) error {
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
	return func(self *Builder) error {
		if caPath == "" || certPath == "" || keyPath == "" {
			return errors.New("invalid TLS configuration")
		}
		for _, path := range certRealPath {
			if _, err := os.Stat(path); err != nil {
				return errors.New("cert file not found: " + path)
			}
		}

		self.clientOption = append(self.clientOption, client.WithTLSClientConfig(
			certRealPath["ca"],
			certRealPath["cert"],
			certRealPath["key"],
		))
		return nil
	}
}

func WithSSH(serverInfo *ssh.ServerInfo, timeout time.Duration) Option {
	return func(self *Builder) error {
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
				return sshconn.New(sshClient, "docker", "system", "dial-stdio")
			},
		}
		self.clientOption = append(self.clientOption, client.WithHTTPClient(&http.Client{Transport: transport}))
		return WithSockProxy()(self)
	}
}

func WithSockProxy() Option {
	return func(self *Builder) error {
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
