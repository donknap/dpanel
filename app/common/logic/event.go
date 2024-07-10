package logic

import (
	"fmt"
	"github.com/docker/docker/api/types/events"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"time"
)

type EventLogic struct {
}

func (self EventLogic) MonitorLoop() {
	messageChan, errorChan := docker.Sdk.Client.Events(docker.Sdk.Ctx, events.ListOptions{})
	for {
		select {
		case message := <-messageChan:
			eventRow := &entity.Event{
				Type:      string(message.Type),
				Action:    string(message.Action),
				Message:   "",
				CreatedAt: time.Unix(message.Time, 0).Format("2006-01-02 15:04:05"),
			}
			switch eventRow.Type + "/" + eventRow.Action {
			case "image/tag", "image/save", "image/push", "image/pull", "image/load",
				"image/import", "image/delete",
				"container/destroy", "container/create",
				"container/stop", "container/start", "container/restart",
				"container/kill", "container/die",
				"container/extract-to-dir":
				eventRow.Message += message.Actor.Attributes["name"]
			case "container/resize":
				eventRow.Message += fmt.Sprintf("%s: %s-%s", message.Actor.Attributes["name"],
					message.Actor.Attributes["width"], message.Actor.Attributes["height"])
			case "volume/mount":
				eventRow.Message += fmt.Sprintf("%s, %s:%s, %s", message.Actor.Attributes["container"],
					message.Actor.Attributes["driver"], message.Actor.Attributes["destination"], message.Actor.Attributes["read/write"])
			case "volume/destroy":
				eventRow.Message += fmt.Sprintf("%s", message.Actor.ID)
			case "network/disconnect", "network/connect":
				eventRow.Message += fmt.Sprintf("%s %s", message.Actor.Attributes["name"],
					message.Actor.Attributes["type"])
			}
			_ = dao.Event.Create(eventRow)
			time.Sleep(time.Second * 1)
		case err := <-errorChan:
			_ = dao.Event.Create(&entity.Event{
				Type:      "error",
				Message:   err.Error(),
				CreatedAt: time.Now().Format(function.ShowYmdHis),
			})
			time.Sleep(time.Second)
		}
	}
}
