package docker

import (
	"context"
	"errors"
	"fmt"
	"github.com/docker/docker/client"
	"github.com/donknap/dpanel/common/service/ssh"
	"github.com/donknap/dpanel/common/service/storage"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"time"
)

var (
	Sdk                        = &Builder{}
	BuilderAuthor              = "DPanel"
	BuildDesc                  = "DPanel is a docker web management panel"
	BuildWebSite               = "https://dpanel.cc"
	BuildVersion               = "1.0.0"
	HostnameTemplate           = "%s.pod.dpanel.local"
	DefaultClientName          = "local"
	ConnectDockerServerTimeout = time.Second * 20
)

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

type ClientDockerInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (self Client) GetDockerEnv() []string {
	runEnv := make([]string, 0)
	if self.EnableTLS {
		runEnv = append(runEnv,
			"DOCKER_TLS_VERIFY=1",
			"DOCKER_CERT_PATH="+filepath.Dir(filepath.Join(storage.Local{}.GetStorageCertPath(), self.TlsCa)),
		)
	}
	runEnv = append(runEnv, fmt.Sprintf("DOCKER_HOST=%s", self.Address))
	return runEnv
}

type Builder struct {
	Name          string
	Client        *client.Client
	Ctx           context.Context
	CtxCancelFunc context.CancelFunc
	runParams     []string
	runEnv        []string
	clientOption  []client.Opt
}

func (self Builder) Close() {
	if self.CtxCancelFunc != nil {
		self.CtxCancelFunc()
	}
	if self.Client != nil {
		_ = self.Client.Close()
	}
}

type Option func(builder *Builder) error

func NewBuilder(opts ...Option) (*Builder, error) {
	c := &Builder{
		Name:      "local",
		runParams: make([]string, 0),
		runEnv:    make([]string, 0),
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
		self.runParams = append(self.runParams, "-H", host)
		self.runEnv = append(self.runEnv, fmt.Sprintf("DOCKER_HOST=%s", host))
		self.clientOption = append(self.clientOption, client.WithHost(host))
		return nil
	}
}

func WithTLS(caPath, certPath, keyPath string) Option {
	certRealPath := map[string]string{
		"ca":   filepath.Join(storage.Local{}.GetStorageCertPath(), caPath),
		"cert": filepath.Join(storage.Local{}.GetStorageCertPath(), certPath),
		"key":  filepath.Join(storage.Local{}.GetStorageCertPath(), keyPath),
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

		self.runParams = append(self.runParams, "--tlsverify",
			"--tlscacert", certRealPath["ca"],
			"--tlscert", certRealPath["cert"],
			"--tlskey", certRealPath["key"],
		)
		self.runEnv = append(self.runEnv,
			"DOCKER_TLS_VERIFY=1",
			"DOCKER_CERT_PATH="+filepath.Dir(certRealPath["ca"]),
		)

		slog.Debug("docker connect tls", "extra params", self.runParams, "env", self.runEnv)
		return nil
	}
}

func WithSSH(serverInfo *ssh.ServerInfo) Option {
	return func(self *Builder) error {
		sshClient, err := ssh.NewClient(ssh.WithServerInfo(serverInfo)...)
		if err != nil {
			return err
		}
		remoteConn, err := sshClient.Conn.Dial("unix", "/var/run/docker.sock")
		if err != nil {
			sshClient.Close()
			return err
		}
		_ = remoteConn.Close()

		localProxySock := filepath.Join(storage.Local{}.GetLocalProxySockPath(), fmt.Sprintf("%s.sock", self.Name))
		_ = os.Remove(localProxySock)
		listener, _ := net.ListenUnix("unix", &net.UnixAddr{Name: localProxySock})

		go func() {
			select {
			case <-self.Ctx.Done():
				_ = listener.Close()
				_ = remoteConn.Close()
				sshClient.Close()
			}
		}()

		go func() {
			for {
				localConn, err := listener.Accept()
				if err != nil {
					slog.Warn("docker proxy sock local close", "err", err)
					return
				}
				go func(lc net.Conn) {
					remoteConn, err = sshClient.Conn.Dial("unix", "/var/run/docker.sock")
					if err != nil {
						slog.Warn("docker proxy sock create remote", "err", err)
						return
					}
					go func() {
						defer remoteConn.Close()
						for {
							buf := make([]byte, 64*1024)
							n, err := lc.Read(buf)
							if err != nil {
								slog.Warn("docker proxy sock local read", "err", err)
								return
							}
							_, err = remoteConn.Write(buf[:n])
							if err != nil {
								slog.Warn("docker proxy sock local to remote", "err", err)
								return
							}
						}
					}()
					go func() {
						for {
							buf := make([]byte, 64*1024)
							n, err := remoteConn.Read(buf)
							if err != nil {
								slog.Warn("docker proxy sock remote read", "err", err)
								return
							}
							_, err = lc.Write(buf[:n])
							if err != nil {
								slog.Warn("docker proxy sock remote to local", "err", err)
								return
							}
						}
					}()
				}(localConn)
			}
		}()
		// 清空掉之前的配置
		self.runParams = make([]string, 0)
		self.runEnv = make([]string, 0)
		self.clientOption = make([]client.Opt, 0)
		return WithAddress("unix://" + localProxySock)(self)
	}
}
