package logic

import (
	"context"
	"github.com/docker/docker/api/types"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/service/docker"
	"time"
)

type EventLogic struct {
}

func (self EventLogic) MonitorLoop() {
	sdk, err := docker.NewDockerClient()
	if err != nil {
		panic(err)
	}
	messageChan, errorChan := sdk.Client.Events(context.Background(), types.EventsOptions{})
	for true {
		select {
		case message := <-messageChan:
			dao.Event.Create(&entity.Event{
				Type:   message.Type,
				Action: message.Action,
				Message: &accessor.EventMessageOption{
					Content: message.Actor.Attributes,
				},
				CreatedAt: time.Unix(message.Time, 0).Format("2006-01-02 15:04:05"),
				Read:      0,
			})
			time.Sleep(time.Second * 1)
		case err := <-errorChan:
			dao.Event.Create(&entity.Event{
				Type: "error",
				Message: &accessor.EventMessageOption{
					Err: err.Error(),
				},
				CreatedAt: time.Now().Format("2006-01-02 15:04:05"),
				Read:      0,
			})
			time.Sleep(time.Second)
		}
	}
}
