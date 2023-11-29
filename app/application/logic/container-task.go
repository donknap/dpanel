package logic

import (
	"context"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/we7coreteam/w7-rangine-go-support/src/facade"
	"log"
	"log/slog"
)

func NewContainerTask() *ContainerTask {
	var obj *ContainerTask
	err := facade.GetContainer().NamedResolve(&obj, "containerTask")
	if err != nil {
		slog.Error(err.Error())
	}
	return obj
}

type CreateMessage struct {
	Name      string
	Image     string
	RunParams *ContainerRunParams
}

type ContainerTask struct {
	QueueCreate chan *CreateMessage
}

func (self *ContainerTask) CreateLoop() {
	for {
		select {
		case message := <-self.QueueCreate:
			log.Printf("build %s from %s starting", message.Name, message.Image)
			sdk, err := docker.NewDockerClient()
			if err != nil {
				fmt.Printf("%v \n", err)
			}
			builder := sdk.GetContainerCreateBuilder()
			builder.WithImage(message.Image)
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
				log.Printf("%v \n", err)
			}
			log.Printf("%v \n", response.ID)
			err = sdk.Client.ContainerStart(context.Background(), response.ID, types.ContainerStartOptions{})
			if err != nil {
				log.Printf("%v \n", err)
			}
		}
	}
}
