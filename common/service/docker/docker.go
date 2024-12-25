package docker

import (
	"context"
	"fmt"
	"github.com/docker/docker/client"
	"github.com/donknap/dpanel/common/service/storage"
	"log/slog"
	"path/filepath"
)

var (
	Sdk              = &Builder{}
	BuilderAuthor    = "DPanel"
	BuildDesc        = "DPanel is a docker web management panel"
	BuildWebSite     = "https://dpanel.cc"
	BuildVersion     = "1.0.0"
	HostnameTemplate = "%s.pod.dpanel.local"
)

type Builder struct {
	Client        *client.Client
	Ctx           context.Context
	CtxCancelFunc context.CancelFunc
	ExtraParams   []string
	Env           []string
	Host          string
}

type NewDockerClientOption struct {
	Host    string // docker 客户端名称
	Address string // docker 客户端地址
	TlsCa   string
	TlsCert string
	TlsKey  string
}

func NewDockerClient(option NewDockerClientOption) (*Builder, error) {
	builder := &Builder{
		ExtraParams: make([]string, 0),
		Env:         make([]string, 0),
	}

	dockerOption := []client.Opt{
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	}

	if option.Address != "" {
		builder.ExtraParams = append(builder.ExtraParams, "-H", option.Address)
		builder.Env = append(builder.Env, fmt.Sprintf("DOCKER_HOST=%s", option.Address))
		dockerOption = append(dockerOption, client.WithHost(option.Address))
	} else {
		option.Host = "local"
	}
	if option.TlsCa != "" && option.TlsCert != "" && option.TlsKey != "" {
		dockerOption = append(dockerOption, client.WithTLSClientConfig(
			filepath.Join(storage.Local{}.GetStorageCertPath(), option.TlsCa),
			filepath.Join(storage.Local{}.GetStorageCertPath(), option.TlsCert),
			filepath.Join(storage.Local{}.GetStorageCertPath(), option.TlsKey),
		))
		builder.ExtraParams = append(builder.ExtraParams, "--tlsverify",
			"--tlscacert", filepath.Join(storage.Local{}.GetStorageCertPath(), option.TlsCa),
			"--tlscert", filepath.Join(storage.Local{}.GetStorageCertPath(), option.TlsCert),
			"--tlskey", filepath.Join(storage.Local{}.GetStorageCertPath(), option.TlsKey))
		builder.Env = append(builder.Env,
			"DOCKER_TLS_VERIFY=1",
			"DOCKER_CERT_PATH="+filepath.Dir(filepath.Join(storage.Local{}.GetStorageCertPath(), option.TlsCa)),
		)
		slog.Debug("docker connect tls", "extra params", builder.ExtraParams, "env", builder.Env)
	}
	obj, err := client.NewClientWithOpts(dockerOption...)
	if err != nil {
		return nil, err
	}
	ctx, cancelFunc := context.WithCancel(context.Background())
	builder.Client = obj
	builder.Ctx = ctx
	builder.CtxCancelFunc = cancelFunc
	builder.Host = option.Host
	return builder, nil
}
