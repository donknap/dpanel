package logic

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/donknap/dpanel/common/service/docker"
	builder "github.com/donknap/dpanel/common/service/docker/image"
	"github.com/donknap/dpanel/common/service/notice"
	"github.com/donknap/dpanel/common/service/ws"
	"io"
	"log/slog"
	"math"
	"strings"
	"time"
)

func (self DockerTask) ImageBuild(task *BuildImageOption) (string, error) {
	_ = notice.Message{}.Info(".imageBuild", "tag", task.Tag)

	b, err := builder.New(
		builder.WithZipFilePath(task.ZipPath),
		builder.WithDockerFileContent(task.DockerFileContent),
		builder.WithDockerFilePath(task.Context),
		builder.WithGitUrl(task.GitUrl),
		builder.WithPlatform(task.Platform),
		builder.WithTag(task.Tag),
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

	wsBuffer := ws.NewProgressPip(fmt.Sprintf(ws.MessageTypeImageBuild, task.ImageId))
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
					if strings.Contains(msg.ErrorDetail.Message, "ADD failed") || strings.Contains(msg.ErrorDetail.Message, "COPY failed") {
						return errors.New("dockerfile 中包含添加文件操作，请使用 Zip 包或是 Git 源码仓库方式创建镜像")
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
