package docker

import (
	"context"
	"errors"
	"fmt"
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
	"github.com/donknap/dpanel/common/service/ssh"
	"github.com/donknap/dpanel/common/service/storage"
)

var (
	Sdk                        = &Builder{}
	BuilderAuthor              = "DPanel"
	BuildDesc                  = "DPanel is a lightweight Docker web management panel"
	BuildWebSite               = "https://dpanel.cc"
	BuildVersion               = "1.0.0"
	DefaultClientName          = "local"
	ConnectDockerServerTimeout = time.Second * 10
)

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
			result = append(result, fmt.Sprintf("DOCKER_HOST=npipe:////./pipe/%s", self.Name))
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
			result = append(result, "-H", fmt.Sprintf("npipe:////./pipe/%s", self.Name))
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

func NewBuilderWithDockerEnv(dockerEnv *Client) (*Builder, error) {
	options := make([]Option, 0)
	options = append(options, WithDockerEnv(dockerEnv))
	options = append(options, WithName(dockerEnv.Name))
	if dockerEnv.EnableTLS {
		options = append(options, WithTLS(dockerEnv.TlsCa, dockerEnv.TlsCert, dockerEnv.TlsKey))
	}
	if dockerEnv.RemoteType == RemoteTypeSSH {
		options = append(options, WithSSH(dockerEnv.SshServerInfo))
	} else {
		options = append(options, WithAddress(dockerEnv.Address))
	}
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
	ctx, cancelFunc := context.WithCancel(context.Background())
	c.Ctx = ctx
	c.CtxCancelFunc = cancelFunc

	for _, opt := range opts {
		err := opt(c)
		if err != nil {
			return nil, err
		}
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

func WithSSH(serverInfo *ssh.ServerInfo) Option {
	return func(self *Builder) error {
		lock := sync.Mutex{}
		transport := &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				lock.Lock()
				//ctx1, _ := context.WithTimeout(ctx, 10*time.Second)
				opts := ssh.WithServerInfo(serverInfo)
				opts = append(opts, ssh.WithContext(ctx))
				sshClient, err := ssh.NewClient(opts...)
				lock.Unlock()
				if err != nil {
					return nil, err
				}
				fmt.Printf("DialContext %v \n", time.Now())
				return sshconn.New(sshClient, "docker", "system", "dial-stdio")
			},
		}
		self.clientOption = append(self.clientOption, client.WithHTTPClient(&http.Client{Transport: transport}))
		return nil
	}
}
