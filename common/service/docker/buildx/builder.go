package buildx

import (
	"bytes"
	"context"
	"fmt"
	"text/template"

	"github.com/docker/docker/api/types/registry"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/docker/types"
	"github.com/donknap/dpanel/common/service/exec"
	"github.com/donknap/dpanel/common/service/exec/local"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
)

func New(ctx context.Context, client *docker.Client, opts ...Option) (*Builder, error) {
	tempFile, _ := storage.Local{}.CreateTempFile("")
	defer func() {
		_ = tempFile.Close()
	}()

	b := &Builder{
		options: &BuildOptions{
			Labels:       make([]string, 0),
			Name:         fmt.Sprintf(define.DockerContextName, client.DockerEnv.Name),
			RegistryAuth: make([]registry.AuthConfig, 0),
		},
	}

	b.ctx, b.ctxCancel = context.WithCancel(ctx)

	var err error
	for _, o := range opts {
		err = o(b)
	}

	return b, err
}

type Builder struct {
	ctx       context.Context
	ctxCancel context.CancelFunc
	options   *BuildOptions
	env       []types.EnvItem
}

func (self Builder) Close() {
	self.ctxCancel()
}

func (self Builder) Execute() (exec.Executor, error) {
	self.options.Labels = append(self.options.Labels,
		"maintainer="+define.PanelAuthor,
		"com.dpanel.description="+define.PanelDesc,
		"com.dpanel.website="+define.PanelWebSite,
		"com.dpanel.version="+facade.GetConfig().GetString("app.version"),
	)

	tmpl, err := template.New("buildx").Funcs(buildShellFunc).Parse(buildShellTmpl)
	if err != nil {
		return nil, err
	}
	var scriptBuffer bytes.Buffer
	if err := tmpl.Execute(&scriptBuffer, self.options); err != nil {
		return nil, err
	}

	env := function.PluckArrayWalk(self.env, func(item types.EnvItem) (string, bool) {
		return item.String(), true
	})
	env = append(env, docker.Sdk.DockerEnv.CommandEnv()...)

	return local.New(
		local.WithCommandName("/bin/sh"),
		local.WithArgs("-c", scriptBuffer.String()),
		local.WithEnv(env),
		local.WithCtx(self.ctx),
	)
}

type BuildOptions struct {
	Name         string // 自动生成的临时名称
	RegistryAuth []registry.AuthConfig
	WorkDir      string // 构建上下文路径 (即最后的 .)

	Annotation []string // --annotation: 为镜像添加 OCI 注解
	BuildArg   []string // --build-arg: 设置构建时变量 (ARG)
	CacheFrom  []string // --cache-from: 外部缓存源 (例如 "user/app:cache")
	CacheTo    []string // --cache-to: 缓存导出目的地 (例如 "type=local,dest=path")
	Labels     []string // --label: 设置镜像的元数据标签
	Outputs    []string // -o, --output: 输出目的地 (格式: "type=local,dest=path")
	Platforms  []string // --platform: 设置构建的目标平台 (如 "linux/amd64")
	Secrets    []string // --secret: 暴露给构建过程的机密信息 (格式: "id=mysecret")

	Builder string               // --builder: 覆盖配置的 builder 实例
	File    string               // -f, --file: Dockerfile 的名称及路
	Target  []BuildOptionsTarget // --target: 设置要构建的目标构建阶段 (Stage)

	NoCache bool // --no-cache: 构建时不使用任何缓存
	Pull    bool // --pull: 始终尝试拉取所有引用的镜像
	Push    bool // --push: Shorthand for "--output=type=registry"
}

type BuildOptionsTarget struct {
	Target string
	Tags   []string
}
