package logic

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/docker/docker/api/types/image"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	builder "github.com/donknap/dpanel/common/service/docker/image"
	"github.com/donknap/dpanel/common/service/notice"
	"github.com/donknap/dpanel/common/service/registry"
	"github.com/donknap/dpanel/common/service/ws"
	"github.com/donknap/dpanel/common/types/define"
	"io"
	"log/slog"
	"math"
	"strings"
	"time"
)

func (self DockerTask) ImageBuild(task *BuildImageOption) (string, error) {
	_ = notice.Message{}.Info(".imageBuild", "tag", task.Tag)

	wsBuffer := ws.NewProgressPip(task.MessageId)
	defer wsBuffer.Close()

	b, err := builder.New(
		builder.WithContext(wsBuffer.Context()),
		builder.WithDockerFilePath(task.BuildDockerfileRoot, task.BuildDockerfileName),
		builder.WithDockerFileContent([]byte(task.BuildDockerfileContent)),
		builder.WithGitUrl(task.BuildGit),
		builder.WithZipFilePath(task.BuildZip),
		builder.WithPlatform(task.Platform),
		builder.WithTag(task.Tag),
		builder.WithArgs(task.BuildArgs...),
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
			msg := docker.BuildMessage{}
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
	_, err = io.Copy(wsBuffer, response.Body)
	if err != nil {
		return log.String(), err
	}
	if task.EnablePush {
		tagDetail := registry.GetImageTagDetail(task.Tag)
		registryConfig := Image{}.GetRegistryConfig(tagDetail.Uri())
		pushResponse, err := docker.Sdk.Client.ImagePush(wsBuffer.Context(), task.Tag, image.PushOptions{
			RegistryAuth: registryConfig.GetAuthString(),
		})
		if err != nil {
			return log.String(), err
		}
		_, err = io.Copy(wsBuffer, pushResponse)
		if err != nil {
			wsBuffer.BroadcastMessage(err.Error())
		}
	}
	return log.String(), nil
}

func (self DockerTask) ImageRemote(w *ws.ProgressPip, r io.ReadCloser) error {
	lastSendTime := time.Now()
	pg := make(map[string]*docker.PullProgress)

	lastJsonStr := new(bytes.Buffer)

	w.OnWrite = func(p string) error {
		if lastJsonStr.Len() > 0 {
			p = lastJsonStr.String() + p
			lastJsonStr.Reset()
		}
		slog.Debug("image pull task", "data", p)
		newReader := bufio.NewReader(bytes.NewReader([]byte(p)))
		pd := docker.BuildMessage{}
		for {
			line, _, err := newReader.ReadLine()
			if err == io.EOF {
				break
			}
			if err := json.Unmarshal(line, &pd); err == nil {
				if pd.ErrorDetail.Message != "" {
					return errors.New(pd.ErrorDetail.Message)
				}
				if pd.Status == "Pulling fs layer" {
					pg[pd.Id] = &docker.PullProgress{
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
				if pd.Status == "Download complete" {
					pg[pd.Id].Downloading = 100
				}
				if pd.Status == "Pull complete" {
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
	return nil
}
