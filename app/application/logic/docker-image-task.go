package logic

import (
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/notice"
	"io"
	"math"
)

func (self DockerTask) ImageBuild(buildImageTask *BuildImageMessage) error {
	go func() {
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
		builder.WithTag(buildImageTask.Tag)
		response, err := builder.Execute()
		if err != nil {
			dao.Image.Where(dao.Image.ID.Eq(buildImageTask.ImageId)).Updates(entity.Image{
				Status:  STATUS_ERROR,
				Message: err.Error(),
			})
			notice.Message{}.Error("imageBuild", err.Error())
			return
		}

		buildProgressMessage := ""

		defer response.Body.Close()
		progressChan := docker.Sdk.Progress(response.Body, fmt.Sprintf("%d", buildImageTask.ImageId))
		for {
			select {
			case message, ok := <-progressChan:
				if !ok {
					notice.Message{}.Success("imageBuild", buildImageTask.Tag)
					dao.Image.Select(dao.Image.Message, dao.Image.Status).Where(dao.Image.ID.Eq(buildImageTask.ImageId)).Updates(entity.Image{
						Status:  STATUS_SUCCESS,
						Message: "",
					})
					return
				}
				if message.Aux != nil && message.Aux.Aux.ID != "" {
					// md5
				}
				if message.Stream != nil {
					buildProgressMessage += message.Stream.Stream
					docker.QueueDockerProgressMessage <- message
				}
				if message.Err != nil {
					dao.Image.Where(dao.Image.ID.Eq(buildImageTask.ImageId)).Updates(entity.Image{
						Status:  STATUS_ERROR,
						Message: buildProgressMessage,
					})
					notice.Message{}.Error("imageBuild", message.Err.Error())
					return
				}
			}
		}
	}()
	return nil
}

func (self DockerTask) ImageRemote(task *ImageRemoteMessage) error {
	go func() {
		var err error
		var out io.ReadCloser
		if task.Type == "pull" {
			out, err = docker.Sdk.Client.ImagePull(docker.Sdk.Ctx, task.Tag, types.ImagePullOptions{
				RegistryAuth: task.Auth,
			})
		} else {
			out, err = docker.Sdk.Client.ImagePush(docker.Sdk.Ctx, task.Tag, types.ImagePushOptions{
				RegistryAuth: task.Auth,
			})
		}
		if err != nil {
			notice.Message{}.Error(task.Type, err.Error())
			return
		}
		pg := make(map[string]*docker.ProgressDownloadImage)
		progressChan := docker.Sdk.Progress(out, task.Tag)
		for {
			select {
			case message, ok := <-progressChan:
				if !ok {
					notice.Message{}.Success(task.Type, task.Tag)
					return
				}
				if message.Detail != nil && message.Detail.Id != "" {
					pd := message.Detail
					if pd.Status == "Pulling fs layer" {
						pg[pd.Id] = &docker.ProgressDownloadImage{
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
					if pd.ProgressDetail.Total > 0 {
						docker.QueueDockerImageDownloadMessage <- pg
					}

				}
				if message.Err != nil {
					notice.Message{}.Error(task.Type, message.Err.Error())
					return
				}
			}
		}
	}()
	return nil
}
