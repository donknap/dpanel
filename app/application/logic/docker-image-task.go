package logic

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/docker/docker/api/types/image"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/notice"
	"github.com/donknap/dpanel/common/service/ws"
	"io"
	"log/slog"
	"math"
	"time"
)

func (self DockerTask) ImageBuild(buildImageTask *BuildImageOption) (string, error) {
	_ = notice.Message{}.Info("imageBuild", "正在构建镜像，请查看控制台输出", buildImageTask.Tag)
	builder := docker.Sdk.GetImageBuildBuilder()
	if buildImageTask.ZipPath != "" {
		builder.WithZipFilePath(buildImageTask.ZipPath)
	}
	if buildImageTask.DockerFileContent != nil {
		builder.WithDockerFileContent(buildImageTask.DockerFileContent)
	}
	if buildImageTask.Context != "" {
		builder.WithDockerFilePath(buildImageTask.Context)
	}
	if buildImageTask.GitUrl != "" {
		builder.WithGitUrl(buildImageTask.GitUrl)
	}
	if buildImageTask.Platform != nil {
		builder.WithPlatform(buildImageTask.Platform.Type, buildImageTask.Platform.Arch)
	}
	builder.WithTag(buildImageTask.Tag)
	response, err := builder.Execute()
	if err != nil {
		return "", err
	}
	defer func() {
		if response.Body.Close() != nil {
			slog.Error("image", "build", err.Error())
		}
	}()

	wsBuffer := ws.NewProgressPip(fmt.Sprintf(ws.MessageTypeImageBuild, buildImageTask.ImageId))
	defer wsBuffer.Close()

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
	_ = notice.Message{}.Success("imageBuild", buildImageTask.Tag)
	return log.String(), nil
}

func (self DockerTask) ImageRemote(task *ImageRemoteOption) error {
	var err error
	var out io.ReadCloser
	if task.Type == "pull" {
		pullOption := image.PullOptions{
			RegistryAuth: task.Auth,
		}
		if task.Platform != "" {
			pullOption.Platform = task.Platform
		}
		tag := task.Tag
		if task.Proxy != "" {
			tag = task.Proxy + "/" + task.Tag
		}
		out, err = docker.Sdk.Client.ImagePull(docker.Sdk.Ctx, tag, pullOption)
	} else {
		out, err = docker.Sdk.Client.ImagePush(docker.Sdk.Ctx, task.Tag, image.PushOptions{
			RegistryAuth: task.Auth,
		})
	}
	if err != nil {
		return err
	}
	defer func() {
		err = out.Close()
		if err != nil {
			slog.Error("image", "pull", err.Error())
		}
	}()

	wsBuffer := ws.NewProgressPip(fmt.Sprintf(ws.MessageTypeImagePull, task.Tag))
	defer wsBuffer.Close()

	lastSendTime := time.Now()
	pg := make(map[string]*docker.PullProgress)

	wsBuffer.OnWrite = func(p string) error {
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
				slog.Error("docker", "image pull task", err)
				return err
			}
		}
		if time.Now().Sub(lastSendTime) < time.Second {
			return nil
		}
		lastSendTime = time.Now()
		wsBuffer.BroadcastMessage(pg)
		return nil
	}
	_, err = io.Copy(wsBuffer, out)
	if err != nil {
		return err
	}
	return nil
}
