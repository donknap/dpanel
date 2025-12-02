package notice

import (
	"context"
	"log/slog"
	"time"

	"github.com/docker/docker/api/types/events"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/docker/types"
	"github.com/donknap/dpanel/common/types/event"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
)

type Processor func(message events.Message)

var Monitor = NewMonitor()

func NewMonitor() *monitor {
	o := &monitor{
		clients:  map[string]*docker.Client{},
		joinChan: make(chan *types.DockerEnv),
	}
	o.ctx, o.ctxCancel = context.WithCancel(context.Background())
	go o.Loop()
	return o
}

type monitor struct {
	ctx       context.Context
	ctxCancel context.CancelFunc
	clients   map[string]*docker.Client
	joinChan  chan *types.DockerEnv
}

func (self *monitor) Close() {
	self.ctxCancel()
	close(self.joinChan)
	for name, c := range self.clients {
		c.Close()
		delete(self.clients, name)
	}
}

func (self *monitor) Loop() {
	for {
		select {
		case <-self.ctx.Done():
			return
		case c, ok := <-self.joinChan:
			if !ok {
				return
			}
			go self.listen(c)
		}
	}
}

func (self *monitor) Join(c *types.DockerEnv) {
	self.joinChan <- c
}

func (self *monitor) Leave(name string) {
	if v, ok := self.clients[name]; ok {
		v.Close()
		delete(self.clients, name)
	}
}

func (self *monitor) listen(env *types.DockerEnv) {
	var initErr error
	var c *docker.Client
	for {
		time.Sleep(5 * time.Second)
		if initErr != nil {
			facade.GetEvent().Publish(event.DockerStopEvent, event.DockerPayload{
				DockerEnv: env,
				Error:     initErr,
			})
		}

		initErr = nil
		c, initErr = docker.NewClientWithDockerEnv(env)
		if initErr != nil {
			continue
		}

		if _, initErr = c.Client.Ping(self.ctx); initErr != nil {
			c.Close()
			continue
		}

		facade.GetEvent().Publish(event.DockerStartEvent, event.DockerPayload{
			DockerEnv: env,
		})

		self.clients[env.Name] = c
		eventChan, errChan := c.Client.Events(context.Background(), events.ListOptions{})

		for {
			select {
			case <-c.Ctx.Done():
				slog.Debug("Monitor closed by client", "name", env.Name)
				goto cleanup
			case <-self.ctx.Done():
				slog.Debug("Monitor closed by monitor", "name", env.Name)
				goto cleanup
			case message, ok := <-eventChan:
				slog.Debug("Monitor message", "name", env.Name, "message", message)
				if !ok {
					goto reconnect
				}
				self.processor(message)
			case err, ok := <-errChan:
				slog.Debug("Monitor error", "name", env.Name, "err", err)
				if !ok {
					goto reconnect
				}
			}
		}
	reconnect:
		c.Close()
		continue
	cleanup:
		c.Close()
		delete(self.clients, env.Name)
	}
}

func (self *monitor) processor(message events.Message) {
	var msg []string
	msgType := string(message.Type) + "/" + string(message.Action)
	switch msgType {
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
		facade.GetEvent().Publish(event.DockerMessageEvent, event.DockerMessagePayload{
			Type:    msgType,
			Message: msg,
		})
	}
}
