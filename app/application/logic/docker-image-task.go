package logic

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"
	"strings"
	"time"

	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker/buildx"
	"github.com/donknap/dpanel/common/service/docker/types"
	"github.com/donknap/dpanel/common/service/ws"
	"github.com/donknap/dpanel/common/types/define"
)

func (self DockerTask) ImageBuild(messageId string, task accessor.ImageSettingOption) (string, error) {
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
		buildx.WithTag(function.PluckArrayWalk(task.Tags, func(item *function.Tag) (string, bool) {
			return item.Uri(), true
		})...),
		buildx.WithBuildArg(task.BuildArgs...),
		buildx.WithPlatform(),
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
	b, err := buildx.New(wsBuffer.Context(), options...)

	//b, err := builder.New(
	//	builder.WithContext(wsBuffer.Context()),
	//	builder.WithDockerFilePath(task.BuildDockerfileName),
	//	builder.WithDockerFileContent([]byte(task.BuildDockerfileContent)),
	//	builder.WithGitUrl(task.BuildGit),
	//	builder.WithZipFilePath(task.BuildDockerfileRoot, task.BuildZip),
	//	builder.WithPlatform(task.PlatformArch),
	//	builder.WithTag(task.Tag),
	//	builder.WithArgs(task.BuildArgs...),
	//)
	if err != nil {
		return "", err
	}
	response, err := b.Execute()
	if err != nil {
		return "", err
	}
	defer func() {
		if response.Close() != nil {
			slog.Error("image", "build", err.Error())
		}
	}()

	out, err := response.RunInPip()
	if err != nil {
		return "", err
	}
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
						return function.ErrorMessage(define.ErrorMessageImageBuildAddFileTypeError)
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
	_, err = io.Copy(wsBuffer, out)
	if err != nil {
		return log.String(), err
	}
	wsBuffer.BroadcastMessage(buffer.String())
	return log.String(), nil
}

func (self DockerTask) ImageRemote(w *ws.ProgressPip, r io.ReadCloser) error {
	if r == nil {
		return function.ErrorMessage(define.ErrorMessageImagePullRegistryBad)
	}
	lastSendTime := time.Now()
	pg := make(map[string]*types.PullProgress)

	lastJsonStr := new(bytes.Buffer)

	w.OnWrite = func(p string) error {
		if lastJsonStr.Len() > 0 {
			p = lastJsonStr.String() + p
			lastJsonStr.Reset()
		}
		if os.Getenv("APP_ENV") == "debug" {
			slog.Debug("image pull task", "data", p)
		}
		newReader := bufio.NewReader(bytes.NewReader([]byte(p)))
		pd := types.BuildMessage{}
		for {
			line, _, err := newReader.ReadLine()
			if err == io.EOF {
				break
			}
			if err := json.Unmarshal(line, &pd); err == nil {
				if pd.ErrorDetail.Message != "" {
					return errors.New(pd.ErrorDetail.Message)
				}
				if pg[pd.Id] == nil {
					pg[pd.Id] = &types.PullProgress{
						Extracting:  0,
						Downloading: 0,
					}
				}
				if pd.ProgressDetail.Total > 0 && pd.Status == "Downloading" {
					pg[pd.Id].Downloading = math.Floor((pd.ProgressDetail.Current / pd.ProgressDetail.Total) * 100)
				}
				if pd.ProgressDetail.Total > 0 && pd.Status == "Extracting" {
					pg[pd.Id].Extracting = math.Floor((pd.ProgressDetail.Current / pd.ProgressDetail.Total) * 100)
				}
				if pd.ProgressDetail.Total > 0 && pd.Status == "Pushing" {
					pg[pd.Id].Downloading = math.Floor((pd.ProgressDetail.Current / pd.ProgressDetail.Total) * 100)
				}
				if pd.Status == "Download complete" {
					pg[pd.Id].Downloading = 100
				}
				if pd.Status == "Pull complete" {
					pg[pd.Id].Extracting = 100
					pg[pd.Id].Downloading = 100
				}
				if pd.Status == "Pushed" || pd.Status == "Layer already exists" {
					pg[pd.Id].Downloading = 100
					pg[pd.Id].Extracting = 100
				}
			} else {
				// 如果 json 解析失败，可能是最后一行 json 被截断了，存到中间变量中，下次再附加上。
				lastJsonStr.Write(line)
				slog.Debug("image pull task json", "error", err)
			}
		}
		if time.Now().Sub(lastSendTime) < time.Second {
			return nil
		}
		lastSendTime = time.Now()
		w.BroadcastMessage(pg)
		return nil
	}
	_, err := io.Copy(w, r)
	if err != nil {
		return err
	}
	w.BroadcastMessage(pg)
	return nil
}
