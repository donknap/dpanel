package logic

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/we7coreteam/w7-rangine-go-support/src/facade"
	"io"
	"log/slog"
	"math"
)

const REGISTER_NAME = "containerTask"

func RegisterContainerTask() {
	err := facade.GetContainer().NamedSingleton(
		REGISTER_NAME, func() *ContainerTask {
			obj := &ContainerTask{}
			obj.QueueCreate = make(chan *CreateMessage, 999)
			obj.stepLog = make(map[int32]*stepMessage)
			return obj
		},
	)
	if err != nil {
		panic(err)
	}
}

func NewContainerTask() *ContainerTask {
	var obj *ContainerTask
	err := facade.GetContainer().NamedResolve(&obj, REGISTER_NAME)
	if err != nil {
		slog.Error(err.Error())
	}
	return obj
}

type CreateMessage struct {
	Name      string
	Image     string
	SiteId    int32
	RunParams *accessor.SiteEnvOption
}

type ContainerTask struct {
	QueueCreate chan *CreateMessage
	stepLog     map[int32]*stepMessage // 用于记录部署任务日志
	sdk         *docker.Builder
}

func (self *ContainerTask) GetTaskStepLog(taskId int32) *stepMessage {
	if stepLog, ok := self.stepLog[taskId]; ok {
		return stepLog
	}
	return nil
}

func (self *ContainerTask) CreateLoop() {
	sdk, err := docker.NewDockerClient()
	if err != nil {
		panic(err)
	}
	self.sdk = sdk

	for {
		select {
		case message := <-self.QueueCreate:
			// 拿到部署任务后，先新建一个任务对象
			// 用于记录进行状态（数据库中）
			// 在本单例对象中建立一个map对象，存放过程中的数据，这些数据不入库
			slog.Info(fmt.Sprintf("run task %d", message.SiteId))
			self.stepLog[message.SiteId] = newStepMessage(message.SiteId)
			self.stepLog[message.SiteId].step(STEP_IMAGE_PULL)
			err = self.pullImage(message)
			if err != nil {
				slog.Info("steplog", self.stepLog)
				self.stepLog[message.SiteId].err(err)
				break
			}

			self.stepLog[message.SiteId].step(STEP_CONTAINER_BUILD)
			builder := sdk.GetContainerCreateBuilder()
			builder.WithImage(message.Image)
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
				self.stepLog[message.SiteId].err(err)
				break
			}

			self.stepLog[message.SiteId].step(STEP_CONTAINER_RUN)
			err = sdk.Client.ContainerStart(context.Background(), response.ID, types.ContainerStartOptions{})
			if err != nil {
				slog.Error(err.Error())
				self.stepLog[message.SiteId].err(err)
				break
			}
			containerInfo, err := sdk.ContainerByField("id", response.ID)
			if err != nil {
				slog.Error(err.Error())
				self.stepLog[message.SiteId].err(err)
				break
			}
			self.stepLog[message.SiteId].syncSiteContainerInfo(containerInfo)
			self.stepLog[message.SiteId].success(response.ID)
			delete(self.stepLog, message.SiteId)
		default:
			for key, _ := range self.stepLog {
				delete(self.stepLog, key)
			}
		}
	}
}

func (self *ContainerTask) pullImage(message *CreateMessage) error {
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
	slog.Info("pull image ", message)
	//尝试拉取镜像
	reader, err := self.sdk.Client.ImagePull(context.Background(), message.Image, types.ImagePullOptions{})
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
			self.stepLog[message.SiteId].process(pg)
		}
	}
	return nil

}
