//go:build windows

package task

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker/build"
	"github.com/donknap/dpanel/common/service/docker/types"
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

	b, err := build.New(
		build.WithContext(wsBuffer.Context()),
		build.WithDockerFilePath(task.BuildDockerfileName),
		build.WithDockerFileContent([]byte(task.BuildDockerfileContent)),
		build.WithGitUrl(task.BuildGit),
		build.WithZipFilePath(task.BuildDockerfileRoot, task.BuildZip),
		build.WithTag(function.PluckArrayWalk(task.Tags, func(item *function.Tag) (string, bool) {
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
	defer func() {
		if response.Body.Close() != nil {
			slog.Error("image", "build", err.Error())
		}
	}()

	log := new(bytes.Buffer)
	buffer := new(bytes.Buffer)

	wsBuffer.OnWrite = func(p string) error {
		newReader := bufio.NewReader(bytes.NewReader([]byte(p)))
		for {
			line, _, err := newReader.ReadLine()
			if err == io.EOF {
				break
			}
			msg := types.BuildMessage{}
			if err = json.Unmarshal(line, &msg); err == nil {
				if msg.ErrorDetail.Message != "" {
					buffer.WriteString(msg.ErrorDetail.Message)
					wsBuffer.BroadcastMessage(buffer.String())
					if strings.Contains(msg.ErrorDetail.Message, "ADD failed") || strings.Contains(msg.ErrorDetail.Message, "COPY failed") {
						return function.ErrorMessage(define.ErrorMessageImageBuildError)
					}
					return errors.New(msg.ErrorDetail.Message)
				} else if msg.PullMessage.Id != "" {
					buffer.WriteString(fmt.Sprintf("\r%s: %s", msg.PullMessage.Id, msg.PullMessage.Progress))
				} else {
					buffer.WriteString(msg.Stream)
				}
			} else {
				slog.Error("docker", "image build task", err, "data", p)
				return err
			}
		}
		log.WriteString(buffer.String())
		if buffer.Len() < 512 {
			return nil
		}
		wsBuffer.BroadcastMessage(buffer.String())
		buffer.Reset()
		return nil
	}
	_, err = io.Copy(wsBuffer, response.Body)
	if err != nil {
		return log.String(), err
	}
	wsBuffer.BroadcastMessage(buffer.String())
	return log.String(), nil
}
