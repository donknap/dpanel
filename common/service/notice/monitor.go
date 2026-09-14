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
	"github.com/donknap/dpanel/common/service/storage"
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
	createdAt    time.Time // 创建时间，用于同名环境更新后，把旧的踢掉
}

func (self *client) Close() {
	self.ctxCancel()
	self.Clear()
	// 因为需要重复的使用监控 client，关掉旧的后，还需要再新建一个新的上下文
	//self.ctx, self.ctxCancel = context.WithCancel(context.Background())
}

func (self *client) Clear() {
	if self.dockerClient != nil {
		self.dockerClient.Close()
	}
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
// TODO: Leave 与 Store 不是原子替换，同名 Join 并发时可能留下未取消的监听；
// dockerClient 在 listen、Close 和 Clients 之间的读写也未同步，后续需一并处理。
func (self *monitor) Join(dockerEnv *types.DockerEnv) {
	self.Leave(dockerEnv.Name)

	c := &client{
		dockerEnv: dockerEnv,
		createdAt: time.Now(),
	}
	c.ctx, c.ctxCancel = context.WithCancel(self.ctx)
	self.clients.Store(dockerEnv.Name, c)

	go self.listen(c)
}

func (self *monitor) Leave(name string) {
	if v, ok := self.clients.LoadAndDelete(name); ok {
		if c, ok := v.(*client); ok {
			c.Close()
		}
	}
	storage.Cache.Delete(fmt.Sprintf(storage.CacheKeyDockerStatus, name))
}

func (self *monitor) Clients() map[string]*docker.Client {
	clients := make(map[string]*docker.Client)
	self.clients.Range(func(key, value interface{}) bool {
		if c, ok := value.(*client); ok && c.dockerClient != nil {
			clients[key.(string)] = c.dockerClient
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
				DockerEnvName: c.dockerEnv.Name,
				Status: types.DockerStatus{
					Message:   fmt.Sprintf("%s, at %s", initErr.Error(), time.Now().Format(define.DateShowYmdHis)),
					Available: false,
				},
			})
			select {
			case <-time.After(10 * time.Second):

			case <-c.ctx.Done():
				slog.Debug("monitor closed by monitor", "name", c.dockerEnv.Name, "error", initErr)
				return
			case <-self.ctx.Done():
				slog.Debug("monitor closed", "error", initErr)
				return
			}
		}
		// 如果数据找不到或是当前循环的环境时间早于存储中的 Clients
		// 时间早说明当前环境已经变更了，不需要再继续循环了
		if v, ok := self.clients.Load(c.dockerEnv.Name); !ok || v.(*client).createdAt.After(c.createdAt) {
			slog.Debug("monitor client not found or updated", "name", c.dockerEnv.Name, "error", initErr)
			c.Close()
			return
		}

		slog.Debug("monitor start", "name", c.dockerEnv, "error", initErr)

		if c.dockerClient, initErr = docker.NewClientWithDockerEnv(c.dockerEnv); initErr != nil {
			c.Clear()
			continue
		}

		pingCtx, pingCancel := context.WithTimeout(c.ctx, define.DockerConnectServerTimeout)
		_, initErr = c.dockerClient.Client.Ping(pingCtx)
		pingCancel()
		if initErr != nil {
			c.Clear()
			if c.ctx.Err() != nil {
				return
			}
			continue
		}
		if c.ctx.Err() != nil {
			c.Clear()
			return
		}

		slog.Debug("monitor publish event", "name", c.dockerEnv.Name)
		facade.GetEvent().Publish(event.DockerDaemonEvent, event.DockerDaemonPayload{
			DockerEnvName: c.dockerEnv.Name,
			Status: types.DockerStatus{
				Message:   "",
				Available: true,
			},
		})

		eventChan, errChan := c.dockerClient.Client.Events(c.ctx, events.ListOptions{})

	eventLoop:
		for {
			select {
			case <-c.ctx.Done():
				slog.Debug("monitor closed by monitor", "name", c.dockerEnv.Name)
				c.Close()
				return
			case <-self.ctx.Done():
				slog.Debug("monitor closed")
				self.Close()
				return
			case message, ok := <-eventChan:
				if !ok {
					initErr = fmt.Errorf("docker event stream closed")
					break eventLoop
				}
				if os.Getenv("APP_ENV") == "debug" {
					if _, _, ok := function.PluckArrayItemWalk(skipActionLog, func(item string) bool {
						return strings.HasPrefix(string(message.Action), item)
					}); !ok {
						message.Actor.Attributes = function.PluckMapWithKeys(message.Actor.Attributes, keepAttribute)
						slog.Debug("monitor message", "name", c.dockerEnv.Name, "message", message)
					}
				}
				self.processor(c.dockerEnv.Name, message)
			case err, ok := <-errChan:
				if c.ctx.Err() != nil || self.ctx.Err() != nil {
					c.Close()
					return
				}
				if !ok || err == nil {
					initErr = fmt.Errorf("docker event stream closed")
				} else {
					initErr = fmt.Errorf("docker event stream failed: %w", err)
				}
				slog.Warn("monitor event stream failed", "name", c.dockerEnv.Name, "error", initErr)
				break eventLoop
			}
		}

		c.Clear()
		c.dockerClient = nil
	}
}

func (self *monitor) processor(name string, message events.Message) {
	facade.GetEvent().Publish(event.DockerMessageEvent, event.DockerMessagePayload{
		DockerEnvName: name,
		Message:       message,
	})
}
