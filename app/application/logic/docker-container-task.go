package logic

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/donknap/dpanel/common/service/docker"
	"io"
	"log/slog"
	"math"
)

type progressDetail struct {
	Id             string `json:"id"`
	Status         string `json:"status"`
	ProgressDetail struct {
		Current float64 `json:"current"`
		Total   float64 `json:"total"`
	} `json:"progressDetail"`
}
type progress struct {
	Downloading float64 `json:"downloading"`
	Extracting  float64 `json:"extracting"`
}

func (self *DockerTask) CreateLoop() {
	self.sdk = docker.Sdk
	for {
		select {
		case message := <-self.QueueCreate:
			// 拿到部署任务后，先新建一个任务对象
			// 用于记录进行状态（数据库中）
			// 在本单例对象中建立一个map对象，存放过程中的数据，这些数据不入库
			slog.Info(fmt.Sprintf("run site id %d", message.SiteId))
			self.containerStepMessage[message.SiteId] = newContainerStepMessage(message.SiteId)
			self.containerStepMessage[message.SiteId].step(STEP_IMAGE_PULL)
			err := self.pullImage(message)
			if err != nil {
				slog.Info("steplog", err.Error())
				self.containerStepMessage[message.SiteId].err(err)
				break
			}

			self.containerStepMessage[message.SiteId].step(STEP_CONTAINER_BUILD)
			builder := docker.Sdk.GetContainerCreateBuilder()
			builder.WithImage(message.RunParams.Image.GetImage())
			builder.WithContext(context.Background())
			builder.WithContainerName(message.Name)
			if message.RunParams.Ports != nil {
				for _, value := range message.RunParams.Ports {
					builder.WithPort(value.Host, value.Dest)
				}
			}
			if message.RunParams.Environment != nil {
				for _, value := range message.RunParams.Environment {
					builder.WithEnv(value.Name, value.Value)
				}
			}
			if message.RunParams.Links != nil {
				for _, value := range message.RunParams.Links {
					if value.Alise == "" {
						value.Alise = value.Name
					}
					builder.WithLink(value.Name, value.Alise)
				}
			}
			if message.RunParams.Volumes != nil {
				for _, value := range message.RunParams.Volumes {
					builder.WithVolume(value.Host, value.Dest)
				}
			}
			builder.WithAlwaysRestart()
			builder.WithPrivileged()
			response, err := builder.Execute()
			if err != nil {
				slog.Error(err.Error())
				self.containerStepMessage[message.SiteId].err(err)
				break
			}
			self.containerStepMessage[message.SiteId].syncSiteContainerInfo(response.ID)

			self.containerStepMessage[message.SiteId].step(STEP_CONTAINER_RUN)
			err = docker.Sdk.Client.ContainerStart(docker.Sdk.Ctx, response.ID, types.ContainerStartOptions{})
			if err != nil {
				slog.Error(err.Error())
				self.containerStepMessage[message.SiteId].err(err)
				break
			}
			if err != nil {
				slog.Error(err.Error())
				self.containerStepMessage[message.SiteId].err(err)
				break
			}
			self.containerStepMessage[message.SiteId].success()
			delete(self.containerStepMessage, message.SiteId)
		}
	}
}

func (self *DockerTask) pullImage(message *CreateMessage) error {
	slog.Info("pull image ", message)
	//尝试拉取镜像
	reader, err := self.sdk.Client.ImagePull(context.Background(), message.RunParams.Image.GetImage(), types.ImagePullOptions{})
	if err != nil {
		return err
	}
	defer reader.Close()

	// 解析进度数据
	pg := make(map[string]*progress)
	out := bufio.NewReader(reader)
	for {
		str, err := out.ReadString('\n')
		if err == io.EOF {
			break
		} else {
			pd := &progressDetail{}
			err = json.Unmarshal([]byte(str), pd)
			if err != nil {
				return err
			}
			if pd.Status == "Pulling fs layer" {
				pg[pd.Id] = &progress{
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
			// 进度信息
			if len(pg) > 0 {
				self.containerStepMessage[message.SiteId].process(pg)
			}
		}
	}
	return nil

}
