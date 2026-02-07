//go:build !windows

package task

import (
	"bytes"
	"io"
	"log/slog"
	"strings"

	"github.com/donknap/dpanel/app/application/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/docker/buildx"
	"github.com/donknap/dpanel/common/service/ws"
	"github.com/donknap/dpanel/common/types/define"
)

func (self Docker) ImageBuild(messageId string, task accessor.ImageSettingOption) (string, error) {
	wsBuffer := ws.NewProgressPip(messageId)
	defer wsBuffer.Close()

	var err error
	defer func() {
		if wsBuffer != nil && err != nil {
			wsBuffer.BroadcastMessage(err.Error())
		}
	}()

	// 如果是 git 指定根目录后在仓库中体现 url#branch:path，Dockerfile 无需要再拼接
	// 如果是 zip 指定根目录后在解包的时候会只保存根目录下的文件，无需要再拼接
	options := []buildx.Option{
		buildx.WithBuildArg(task.BuildArgs...),
		buildx.WithBuildSecret(task.BuildSecret...),
		buildx.WithPlatform(task.BuildPlatformType...),
		buildx.WithOutputImage(task.BuildEnablePush, ""),
	}
	options = append(options, buildx.WithTag(function.PluckArrayWalk(task.Tags, func(item *function.Tag) (string, bool) {
		// 获取仓库权限
		if v := (logic.Image{}).GetRegistryConfig(item.Registry); v != nil {
			options = append(options, buildx.WithRegistryAuth(v.Config))
		}
		return item.Uri(), true
	})...))

	if task.BuildCacheType != "" {
		options = append(options, buildx.WithCache(task.BuildCacheType))
	}

	if task.BuildGit != "" {
		options = append(options, buildx.WithDockerFilePath(task.BuildDockerfileName))
		options = append(options, buildx.WithGitUrl(task.BuildGit))
	} else if task.BuildZip != "" {
		options = append(options, buildx.WithDockerFilePath(task.BuildDockerfileName))
		options = append(options, buildx.WithWorkDir(task.BuildDockerfileRoot))
		options = append(options, buildx.WithZipFilePath(task.BuildZip))
	} else if task.BuildDockerfileName != "" {
		options = append(options, buildx.WithDockerFilePath(task.BuildDockerfileName))
	} else {
		options = append(options, buildx.WithDockerFileContent([]byte(task.BuildDockerfileContent)))
	}
	b, err := buildx.New(wsBuffer.Context(), docker.Sdk, options...)
	if err != nil {
		return "", err
	}
	cmd, err := b.Execute()
	if err != nil {
		return "", err
	}
	defer func() {
		if cmd.Close() != nil {
			slog.Error("image", "build", err.Error())
		}
	}()
	out, err := cmd.RunInPip()
	if err != nil {
		return "", err
	}
	log := new(bytes.Buffer)
	wsBuffer.OnWrite = func(p string) error {
		log.WriteString(p)
		wsBuffer.BroadcastMessage(p)
		return nil
	}
	_, err = io.Copy(wsBuffer, out)
	if err != nil {
		return log.String(), function.ErrorMessage(define.ErrorMessageCommonCancelOperator, "message", err.Error())
	}
	if strings.Contains(log.String(), "ERROR: no builder") {
		return log.String(), function.ErrorMessage(define.ErrorMessageImageBuildError, "message", log.String())
	}
	// 检测是否成功
	if err = b.Result(); err != nil {
		return log.String(), function.ErrorMessage(define.ErrorMessageImageBuildError, "message", "")
	}
	return log.String(), nil
}
