package notice

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types/events"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/docker/types"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/donknap/dpanel/common/types/event"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
)

// 跳过这些重复的事件
var skipActionLog = []string{
	"exec_create",
	"exec_die",
}

// 只记录这些属性值
var keepAttribute = []string{
	"name", "image",
}

var Monitor = NewMonitor()

func NewMonitor() *monitor {
	o := &monitor{
		clients: sync.Map{},
	}
	o.ctx, o.ctxCancel = context.WithCancel(context.Background())
	return o
}

type client struct {
	dockerEnv    *types.DockerEnv
	dockerClient *docker.Client
	ctx          context.Context
	ctxCancel    context.CancelFunc
}

func (self *client) Close() {
	self.ctxCancel()
	if self.dockerClient != nil {
		self.dockerClient.Close()
	}
	// 因为需要重复的使用监控 client，关掉旧的后，还需要再新建一个新的上下文
	self.ctx, self.ctxCancel = context.WithCancel(context.Background())
}

type monitor struct {
	ctx       context.Context
	ctxCancel context.CancelFunc
	clients   sync.Map // 监控的实体对像 *client
}

func (self *monitor) Close() {
	self.ctxCancel()
	self.clients.Range(func(key, value interface{}) bool {
		if v, ok := value.(*client); ok {
			v.Close()
		}
		return true
	})
}

// Join 在加入时，首先检查之前是否存在，如果存在也强制退出，适用于编辑更新配置时
func (self *monitor) Join(dockerEnv *types.DockerEnv) {
	self.Leave(dockerEnv.Name)

	c := &client{
		dockerEnv: dockerEnv,
	}
	c.ctx, c.ctxCancel = context.WithCancel(context.Background())
	self.clients.Store(dockerEnv.Name, c)

	go self.listen(c)
}

func (self *monitor) Leave(name string) {
	if v, ok := self.clients.LoadAndDelete(name); ok {
		if c, ok := v.(*client); ok {
			c.Close()
		}
	}
}

func (self *monitor) Clients() map[string]*docker.Client {
	clients := make(map[string]*docker.Client)
	self.clients.Range(func(key, value interface{}) bool {
		if client, ok := value.(*docker.Client); ok {
			clients[key.(string)] = client
		}
		return true
	})
	return clients
}

func (self *monitor) listen(c *client) {
	var initErr error

	for {
		if initErr != nil {
			facade.GetEvent().Publish(event.DockerDaemonEvent, event.DockerDaemonPayload{
				DockerEnv: c.dockerEnv,
				Status: types.DockerStatus{
					Message:   fmt.Sprintf("%s, at %s", initErr.Error(), time.Now().Format(define.DateShowYmdHis)),
					Available: false,
				},
			})
		}
		time.Sleep(10 * time.Second)
		if _, ok := self.clients.Load(c.dockerEnv.Name); !ok {
			slog.Debug("Monitor client not found", "name", c.dockerEnv.Name, "error", initErr)
			c.Close()
			return
		}

		if os.Getenv("APP_ENV") == "debug" {
			slog.Debug("Monitor start", "name", c.dockerEnv.Name, "error", initErr)
		}

		if c.dockerClient, initErr = docker.NewClientWithDockerEnv(c.dockerEnv); initErr != nil {
			c.Close()
			continue
		}

		if _, initErr = c.dockerClient.Client.Ping(self.ctx); initErr != nil {
			c.Close()
			continue
		}

		facade.GetEvent().Publish(event.DockerDaemonEvent, event.DockerDaemonPayload{
			DockerEnv: c.dockerEnv,
			Status: types.DockerStatus{
				Message:   "",
				Available: true,
			},
		})

		eventChan, errChan := c.dockerClient.Client.Events(context.Background(), events.ListOptions{})

	eventLoop:
		for {
			select {
			case <-c.ctx.Done():
				slog.Debug("Monitor closed by monitor", "name", c.dockerEnv.Name)
				c.Close()
				break eventLoop
			case <-self.ctx.Done():
				slog.Debug("Monitor closed")
				self.Close()
				return
			case message, ok := <-eventChan:
				if os.Getenv("APP_ENV") == "debug" {
					if _, _, ok := function.PluckArrayItemWalk(skipActionLog, func(item string) bool {
						return strings.HasPrefix(string(message.Action), item)
					}); !ok {
						message.Actor.Attributes = function.PluckMapWithKeys(message.Actor.Attributes, keepAttribute)
						slog.Debug("Monitor message", "name", c.dockerEnv.Name, "message", message)
					}
				}
				if !ok {
					break eventLoop
				}
				self.processor(c.dockerEnv.Name, message)
			case err, ok := <-errChan:
				if !ok {
					slog.Debug("Monitor error", "name", c.dockerEnv.Name, "error", err)
					break eventLoop
				}
			}
		}
	}
}

func (self *monitor) processor(name string, message events.Message) {
	var msg []string
	msgType := string(message.Type) + "/" + string(message.Action)
	switch msgType {
	//case "image/tag", "image/save", "image/push", "image/pull", "image/load",
	//	"image/import", "image/delete",
	//	"container/destroy", "container/create",
	//	"container/stop", "container/start", "container/restart",
	//	"container/kill", "container/die",
	//	"container/extract-to-dir":
	//	msg = []string{
	//		message.Actor.Attributes["name"],
	//	}
	//case "container/resize":
	//	msg = []string{
	//		message.Actor.Attributes["name"], ":",
	//		message.Actor.Attributes["width"], "-", message.Actor.Attributes["height"],
	//	}
	//case "volume/mount":
	//	msg = []string{
	//		"container", message.Actor.Attributes["container"],
	//		"mount", message.Actor.Attributes["destination"],
	//		"driver", message.Actor.Attributes["driver"],
	//		"permission", message.Actor.Attributes["read/write"],
	//	}
	//case "network/disconnect", "network/connect":
	//	msg = []string{
	//		"container", message.Actor.Attributes["container"][:12],
	//		string(message.Action),
	//		message.Actor.Attributes["name"],
	//		"type", message.Actor.Attributes["type"],
	//	}
	case "container/destroy", "container/create",
		"container/stop", "container/start", "container/restart",
		"container/kill", "container/die":
		msg = []string{
			message.Actor.Attributes["name"],
		}
		//case "volume/destroy":
		//	msg = []string{
		//		message.Actor.ID,
		//	}
	}
	if msg != nil {
		facade.GetEvent().Publish(event.DockerMessageEvent, event.DockerMessagePayload{
			Type:    name,
			Action:  msgType,
			Message: msg,
			Time:    message.TimeNano,
		})
	}
}
