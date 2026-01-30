package buildx

import (
	"context"
	"strconv"

	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/docker/types"
	"github.com/donknap/dpanel/common/service/exec"
	"github.com/donknap/dpanel/common/service/exec/local"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
)

const (
	DPanelBuilder = "dpanel-builder"
)

func New(ctx context.Context, opts ...Option) (*Builder, error) {
	b := &Builder{
		options: &BuildOptions{
			Labels: make([]string, 0),
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
	workDir   string
	env       []types.EnvItem
}

func (self Builder) Close() {
	self.ctxCancel()
}

func (self Builder) Execute() (exec.Executor, error) {
	cmd := make([]string, 0)
	cmd = append(cmd, docker.Sdk.DockerEnv.CommandParams()...)
	cmd = append(cmd, "buildx",
		"build",
		"--progress", "plain",
		"--builder", DPanelBuilder,
	)
	appendArgv := func(flag string, values []string) {
		for _, v := range values {
			cmd = append(cmd, flag, v)
		}
	}
	appendArgv("-t", self.options.Tags)
	appendArgv("--build-arg", self.options.BuildArg)
	appendArgv("--cache-from", self.options.CacheFrom)
	appendArgv("--cache-to", self.options.CacheTo)

	self.options.Labels = append(self.options.Labels,
		"maintainer="+define.PanelAuthor,
		"com.dpanel.description="+define.PanelDesc,
		"com.dpanel.website="+define.PanelWebSite,
		"com.dpanel.version="+facade.GetConfig().GetString("app.version"),
	)
	appendArgv("--label", function.PluckArrayWalk(self.options.Labels, func(item string) (string, bool) {
		return strconv.Quote(item), true
	}))
	appendArgv("--annotation", self.options.Annotation)
	appendArgv("--platform", self.options.Platforms)
	appendArgv("--secret", self.options.Secrets)
	appendArgv("--output", self.options.Outputs)

	if self.options.File != "" {
		cmd = append(cmd, "-f", self.options.File)
	}

	if self.options.Target != "" {
		cmd = append(cmd, "--target", self.options.Target)
	}

	if self.options.NoCache {
		cmd = append(cmd, "--no-cache")
	}

	if self.options.Pull {
		cmd = append(cmd, "--pull")
	}

	if self.options.Push {
		cmd = append(cmd, "--push")
	}

	cmd = append(cmd, self.workDir)

	return local.New(
		local.WithCommandName("docker"),
		local.WithArgs(cmd...),
	)
}

type BuildOptions struct {
	Annotation []string // --annotation: 为镜像添加 OCI 注解
	BuildArg   []string // --build-arg: 设置构建时变量 (ARG)
	CacheFrom  []string // --cache-from: 外部缓存源 (例如 "user/app:cache")
	CacheTo    []string // --cache-to: 缓存导出目的地 (例如 "type=local,dest=path")
	Labels     []string // --label: 设置镜像的元数据标签
	Outputs    []string // -o, --output: 输出目的地 (格式: "type=local,dest=path")
	Platforms  []string // --platform: 设置构建的目标平台 (如 "linux/amd64")
	Secrets    []string // --secret: 暴露给构建过程的机密信息 (格式: "id=mysecret")
	Tags       []string // -t, --tag: 镜像名称及标签 (格式: "name:tag")

	Builder string // --builder: 覆盖配置的 builder 实例
	File    string // -f, --file: Dockerfile 的名称及路
	Target  string // --target: 设置要构建的目标构建阶段 (Stage)

	Load    bool // --load: Shorthand for "--output=type=docker"
	NoCache bool // --no-cache: 构建时不使用任何缓存
	Pull    bool // --pull: 始终尝试拉取所有引用的镜像
	Push    bool // --push: Shorthand for "--output=type=registry"
}
