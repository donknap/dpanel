package docker

import (
	"context"
	"errors"
	"fmt"
	"github.com/docker/docker/client"
	"github.com/donknap/dpanel/common/service/storage"
	"log/slog"
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
	EnableSSH         bool              `json:"enableSSH"`
	SshUsername       string            `json:"sshUsername"`
	SshPassword       string            `json:"sshPassword"`
	SshPort           int               `json:"sshPort"`
	SshKey            string            `json:"sshKey"`
	SshType           string            `json:"sshType"`
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
	_ = self.Client.Close()
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
	for _, opt := range opts {
		err := opt(c)
		if err != nil {
			return nil, err
		}
	}
	obj, err := client.NewClientWithOpts(c.clientOption...)
	if err != nil {
		return nil, err
	}
	ctx, cancelFunc := context.WithCancel(context.Background())
	c.Client = obj
	c.Ctx = ctx
	c.CtxCancelFunc = cancelFunc
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
