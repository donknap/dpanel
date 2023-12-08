package logic

import (
	"context"
	"encoding/json"
	"github.com/docker/docker/api/types"
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
			actor, _ := json.Marshal(message.Actor.Attributes)
			dao.Event.Create(&entity.Event{
				Type:      message.Type,
				CreatedAt: time.Unix(message.Time, 0).Format("2006-01-02 15:04:05"),
				Message:   string(actor),
			})
			time.Sleep(time.Second * 5)
		case err := <-errorChan:
			dao.Event.Create(&entity.Event{
				Type:      "error",
				CreatedAt: time.Now().Format("2006-01-02 15:04:05"),
				Message:   err.Error(),
			})
			time.Sleep(time.Second)
		}
	}
}
