package buildx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	exec2 "os/exec"
	"regexp"
	"strings"
	"text/template"

	"github.com/docker/docker/api/types/registry"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/docker/types"
	"github.com/donknap/dpanel/common/service/exec"
	"github.com/donknap/dpanel/common/service/exec/local"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
)

func New(ctx context.Context, sdk *docker.Client, opts ...Option) (*Builder, error) {
	if sdk == nil || sdk.Client == nil || sdk.DockerEnv == nil {
		return nil, errors.New("Docker client is required for buildx")
	}
	b := &Builder{
		sdk: sdk,
		options: &BuildOptions{
			Labels:       make([]string, 0),
			Builder:      fmt.Sprintf(define.DockerBuilderName, sdk.Name),
			RegistryAuth: make([]*registry.AuthConfig, 0),
		},
	}

	b.ctx, b.ctxCancel = context.WithCancel(ctx)
	var err error
	for _, o := range opts {
		if err = b.ctx.Err(); err != nil {
			b.Close()
			return nil, err
		}
		if err = o(b); err != nil {
			b.Close()
			return nil, err
		}
	}
	return b, nil
}

type Builder struct {
	sdk       *docker.Client
	ctx       context.Context
	ctxCancel context.CancelFunc
	options   *BuildOptions
	env       []types.EnvItem
	tempDirs  []string
}

func (self Builder) Close() {
	self.ctxCancel()
	for _, dir := range self.tempDirs {
		_ = os.RemoveAll(dir)
	}
}

func (self *Builder) Execute() (exec.Executor, error) {
	self.options.Labels = append(self.options.Labels,
		"maintainer="+define.PanelAuthor,
		"com.dpanel.description="+define.PanelDesc,
		"com.dpanel.website="+define.PanelWebSite,
		"com.dpanel.version="+facade.GetConfig().GetString("app.version"),
	)
	env := make([]string, 0, len(self.env))
	for _, item := range self.env {
		env = append(env, item.String())
	}
	tmpl, err := template.New("buildx").Funcs(template.FuncMap{"quote": shellQuote}).Parse(buildShellTmpl)
	if err != nil {
		return nil, err
	}
	var script strings.Builder
	if err := tmpl.Execute(&script, self.options); err != nil {
		return nil, err
	}
	if self.options.Push {
		for _, auth := range self.options.RegistryAuth {
			cmd := exec2.CommandContext(self.ctx, "docker", "login", auth.ServerAddress, "-u", auth.Username, "--password-stdin")
			cmd.Env = self.sdk.DockerEnv.CommandEnv()
			cmd.Stdin = strings.NewReader(auth.Password + "\n")
			if err := cmd.Run(); err != nil {
				return nil, fmt.Errorf("docker login to registry %q: %w", auth.ServerAddress, err)
			}
		}
	}
	return local.New(
		local.WithCommandName("/bin/sh"),
		local.WithArgs("-c", script.String()),
		local.WithEnv(append(env, self.sdk.DockerEnv.CommandEnv()...)),
		local.WithQuiet(),
		local.WithCtx(self.ctx),
		local.WithIndependentProcessGroup(),
		local.WithKillProcessGroupOnCancel(),
	)
}

func (self *Builder) Run(output io.Writer) (string, string, error) {
	cmd, err := self.Execute()
	if err != nil {
		return "", "", err
	}
	defer cmd.Close()
	pipe, err := cmd.RunInPip()
	if err != nil {
		return "", "", err
	}
	defer pipe.Close()
	_ = pipe.CloseWrite()
	var log bytes.Buffer
	if output == nil {
		output = io.Discard
	}
	_, err = io.Copy(io.MultiWriter(&log, output), pipe)
	if err != nil {
		return log.String(), "", err
	}
	if strings.Contains(log.String(), "ERROR: no builder") {
		return log.String(), "", errors.New("buildx builder is unavailable")
	}
	matches := regexp.MustCompile(`"containerimage\.digest"\s*:\s*"(sha256:[a-f0-9]+)"`).FindAllStringSubmatch(log.String(), -1)
	imageIDs := make([]string, 0, len(matches))
	for _, match := range matches {
		imageIDs = append(imageIDs, match[1])
	}
	if len(imageIDs) == 0 {
		return log.String(), "", errors.New("buildx output did not include an image digest")
	}
	return log.String(), strings.Join(imageIDs, "-"), nil
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
