package task

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"path"
	"strings"

	"github.com/donknap/dpanel/app/application/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/docker/build"
	"github.com/donknap/dpanel/common/service/docker/buildx"
	"github.com/donknap/dpanel/common/service/docker/types"
	"github.com/donknap/dpanel/common/service/ws"
	"github.com/donknap/dpanel/common/types/define"
)

func (self Docker) ImageBuildX(messageId string, task accessor.ImageSettingOption) (string, error) {
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
	for _, tag := range task.Tags {
		if !tag.Enable {
			continue
		}
		if v := (logic.Image{}).GetRegistryConfig(tag.Registry); v != nil {
			options = append(options, buildx.WithRegistryAuth(v.Config))
		}
		options = append(options, buildx.WithTag(tag.Target, tag.Uri()))
	}

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
	} else if task.BuildPath != "" {
		options = append(options, buildx.WithDockerFilePath(path.Join(task.BuildPath, task.BuildDockerfileName)))
		options = append(options, buildx.WithWorkDir(task.BuildPath))
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
		b.Close()
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

	return log.String(), nil
}

func (self Docker) ImageBuild(sdk *docker.Client, messageId string, task accessor.ImageSettingOption) (string, error) {
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
	b, err := build.New(
		build.WithSdk(sdk),
		build.WithContext(wsBuffer.Context()),
		build.WithDockerFilePath(task.BuildDockerfileName),
		build.WithDockerFileContent([]byte(task.BuildDockerfileContent)),
		build.WithGitUrl(task.BuildGit),
		build.WithZipFilePath(task.BuildDockerfileRoot, task.BuildZip),
		build.WithTag(function.PluckArrayWalk(task.Tags, func(item accessor.ImageSettingTag) (string, bool) {
			return item.Uri(), true
		})...),
		build.WithArgs(task.BuildArgs...),
	)
	if err != nil {
		return "", err
	}
	response, err := b.Execute()
	if err != nil {
		return "", err
	}
	go func() {
		<-wsBuffer.Done()
		_ = response.Body.Close()
		err = sdk.Client.BuildCancel(sdk.Ctx, b.GetBuildId())
		if err != nil {
			slog.Error("image build cancel", "error", err.Error())
		}
	}()
	defer func() {
		if response.Body.Close() != nil {
			slog.Error("image", "build", err.Error())
		}
	}()

	log := new(bytes.Buffer)
	wsBuffer.OnWrite = func(p string) error {
		log.WriteString(p)
		newReader := bufio.NewReader(bytes.NewReader([]byte(p)))
		for {
			line, _, err := newReader.ReadLine()
			if err == io.EOF {
				break
			}
			msg := types.BuildMessage{}
			if err = json.Unmarshal(line, &msg); err == nil {
				if msg.ErrorDetail.Message != "" {
					wsBuffer.BroadcastMessage(msg.ErrorDetail.Message)
				} else if msg.PullMessage.Id != "" {
					wsBuffer.BroadcastMessage(fmt.Sprintf("\r%s: %s", msg.PullMessage.Id, msg.PullMessage.Progress))
				} else {
					wsBuffer.BroadcastMessage(msg.Stream)
				}
			} else {
				slog.Error("docker", "image build task", err, "data", p)
				return err
			}
		}
		return nil
	}
	_, err = io.Copy(wsBuffer, response.Body)
	if err != nil {
		return log.String(), function.ErrorMessage(define.ErrorMessageCommonCancelOperator, "message", err.Error())
	}
	if !strings.Contains(log.String(), "Successfully built") {
		return log.String(), function.ErrorMessage(define.ErrorMessageImageBuildError, "message", "")
	}
	return log.String(), nil
}
