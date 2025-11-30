package logic

import (
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types/events"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
)

func NewEventLogin() *EventLogic {
	return &EventLogic{
		dataPool: make([]*entity.Event, 0),
		ticker:   time.NewTicker(10 * time.Second),
	}
}

type EventLogic struct {
	dataPool []*entity.Event
	mu       sync.Mutex
	max      int
	ticker   *time.Ticker
}

func (self *EventLogic) Close() {
	slog.Debug("Event monitor done", "name", docker.Sdk.Name)
	self.commit()
	self.ticker.Stop()
	self.dataPool = []*entity.Event{}
}

func (self *EventLogic) MonitorLoop() {
	slog.Debug("Event monitor start", "name", docker.Sdk.Name)

	go func() {
		for {
			select {
			case <-docker.Sdk.Ctx.Done():
				self.Close()
				return
			case <-self.ticker.C:
				self.commit()
			}
		}
	}()

	messageChan, errorChan := docker.Sdk.Client.Events(docker.Sdk.Ctx, events.ListOptions{})
	for {
		select {
		case <-docker.Sdk.Ctx.Done():
			slog.Debug("Event monitor close")
			self.Close()
			return
		case message, ok := <-messageChan:
			slog.Debug("Event monitor catch", "message", message, "quick", ok)
			if !ok {
				return
			}
			var msg []string
			switch string(message.Type) + "/" + string(message.Action) {
			case "image/tag", "image/save", "image/push", "image/pull", "image/load",
				"image/import", "image/delete",
				"container/destroy", "container/create",
				"container/stop", "container/start", "container/restart",
				"container/kill", "container/die",
				"container/extract-to-dir":
				msg = []string{
					message.Actor.Attributes["name"],
				}
			case "container/resize":
				msg = []string{
					message.Actor.Attributes["name"], ":",
					message.Actor.Attributes["width"], "-", message.Actor.Attributes["height"],
				}
			case "volume/mount":
				msg = []string{
					"container", message.Actor.Attributes["container"],
					"mount", message.Actor.Attributes["destination"],
					"driver", message.Actor.Attributes["driver"],
					"permission", message.Actor.Attributes["read/write"],
				}
			case "volume/destroy":
				msg = []string{
					message.Actor.ID,
				}
			case "network/disconnect", "network/connect":
				msg = []string{
					"container", message.Actor.Attributes["container"][:12],
					string(message.Action),
					message.Actor.Attributes["name"],
					"type", message.Actor.Attributes["type"],
				}
			}
			if msg != nil {
				self.mu.Lock()
				self.dataPool = append(self.dataPool, &entity.Event{
					Type:      string(message.Type),
					Action:    string(message.Action),
					Message:   strings.Join(msg, " "),
					CreatedAt: time.Unix(message.Time, 0).In(time.Local).Format("2006-01-02 15:04:05"),
				})
				self.mu.Unlock()
			}
		case err, ok := <-errorChan:
			slog.Debug("Event monitor catch", "err", err, "quick", ok)
			if !ok {
				return
			}
			if err != nil {
				self.mu.Lock()
				self.dataPool = append(self.dataPool, &entity.Event{
					Type:      "error",
					Message:   err.Error(),
					CreatedAt: time.Now().Format(define.DateShowYmdHis),
				})
				self.mu.Unlock()
			}
		}
	}
}

func (self *EventLogic) commit() {
	if len(self.dataPool) == 0 {
		return
	}
	slog.Debug("Event monitor commit start", "length", len(self.dataPool))

	self.mu.Lock()
	defer self.mu.Unlock()

	db, err := facade.GetDbFactory().Channel("default")
	if err != nil {
		slog.Debug("Event monitor commit", "err", err)
		return
	}

	err = db.CreateInBatches(self.dataPool, len(self.dataPool)).Error
	if err != nil {
		slog.Debug("Event monitor commit", "len", len(self.dataPool), "err", err)
		return
	}
	self.dataPool = []*entity.Event{}
	return
}
