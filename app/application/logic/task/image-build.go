package task

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"strings"

	"github.com/donknap/dpanel/app/application/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/docker/build"
	"github.com/donknap/dpanel/common/service/docker/types"
	"github.com/donknap/dpanel/common/service/ws"
	"github.com/donknap/dpanel/common/types/define"
)

func (self Docker) Build(sdk *docker.Client, wsBuffer *ws.ProgressPip, task accessor.ImageSettingOption) (string, string, error) {

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
		return "", "", err
	}
	defer func() {
		if closeErr := b.Close(); closeErr != nil {
			slog.Warn("image build context cleanup", "error", closeErr)
		}
	}()
	stopBuildCancel := context.AfterFunc(wsBuffer.Context(), func() {
		if err := sdk.Client.BuildCancel(sdk.Ctx, b.GetBuildId()); err != nil {
			slog.Error("image build cancel", "error", err)
		}
	})
	defer stopBuildCancel()
	response, err := b.Execute()
	if err != nil {
		return "", "", err
	}
	stopResponseClose := context.AfterFunc(wsBuffer.Context(), func() {
		_ = response.Body.Close()
	})
	defer stopResponseClose()
	defer func() {
		if closeErr := response.Body.Close(); closeErr != nil {
			slog.Error("image build response close", "error", closeErr)
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
			msg := types.ImageProgress{}
			if err = json.Unmarshal(line, &msg); err == nil {
				if msg.ErrorDetail.Message != "" {
					wsBuffer.BroadcastMessage(msg.ErrorDetail.Message)
				} else if msg.Id != "" {
					wsBuffer.BroadcastMessage(fmt.Sprintf("\r%s: %s", msg.Id, msg.Progress))
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
		return log.String(), "", function.ErrorMessage(define.ErrorMessageCommonCancelOperator, "message", err.Error())
	}
	if !strings.Contains(log.String(), "Successfully built") {
		return log.String(), "", function.ErrorMessage(define.ErrorMessageImageBuildError, "message", "")
	}
	matches := regexp.MustCompile(`Successfully built\s*([a-f0-9]+)`).FindAllStringSubmatch(log.String(), -1)
	imageID := strings.Join(function.PluckArrayWalk(matches, func(item []string) (string, bool) {
		return item[1], true
	}), "-")
	if task.BuildEnablePush {
		for _, tag := range task.Tags {
			if err = wsBuffer.Context().Err(); err != nil {
				return log.String(), imageID, err
			}
			registryConfig := logic.Image{}.GetRegistryConfig(tag.Registry)
			wsBuffer.BroadcastMessage("\r\nPushing " + tag.Uri() + "\r\n")
			err = sdk.ImagePush(wsBuffer.Context(), tag.Uri(), docker.ImagePushOption{
				Registry: *registryConfig,
				OnProgress: func(items map[string]*types.PullProgress) {
					for layer, item := range items {
						if item != nil {
							wsBuffer.BroadcastMessage(fmt.Sprintf("\r%s: %.0f%%", layer, max(item.Downloading, item.Extracting)))
						}
					}
				},
			})
			if err != nil {
				return log.String(), imageID, err
			}
		}
	}
	return log.String(), imageID, nil
}
