package notice

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types/events"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/docker/types"
	"github.com/donknap/dpanel/common/service/ssh"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/service/tunnel"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/donknap/dpanel/common/types/event"
	"github.com/patrickmn/go-cache"
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

func (self *client) checkConnection() (*bool, error) {
	var address string
	switch self.dockerEnv.RemoteType {
	case define.DockerRemoteTypeTcp:
		endpoint, err := url.Parse(self.dockerEnv.Address)
		if err != nil || endpoint.Scheme != "tcp" || endpoint.Host == "" {
			return nil, errors.New("invalid TCP Docker address")
		}
		address = endpoint.Host
	case define.DockerRemoteTypeSSH:
		info := self.dockerEnv.SshServerInfo
		if info == nil || info.Address == "" {
			return nil, errors.New("invalid SSH server address")
		}
		port := info.Port
		if port == 0 {
			port = 22
		}
		address = net.JoinHostPort(strings.Trim(info.Address, "[]"), strconv.Itoa(port))
	default:
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(self.ctx, define.DockerConnectServerTimeout)
	defer cancel()
	dialContext := (&net.Dialer{}).DialContext
	if self.dockerEnv.Tunnel != nil {
		client, err := ssh.NewClient(
			ssh.WithServerInfo(self.dockerEnv.Tunnel.SSH),
			ssh.WithConnectContext(ctx),
		)
		if err != nil {
			return function.Ptr(false), err
		}
		t, err := tunnel.New(tunnel.WithSSH(client))
		if err != nil {
			return function.Ptr(false), err
		}
		defer t.Close()
		dialContext = t.DialContext
	}
	conn, err := dialContext(ctx, "tcp", address)
	if err != nil {
		return function.Ptr(false), err
	}
	_ = conn.Close()
	return function.Ptr(true), nil
}

type monitor struct {
	mu        sync.Mutex
	ctx       context.Context
	ctxCancel context.CancelFunc
	clients   sync.Map // 监控的实体对像 *client
}

func (self *monitor) Close() {
	self.mu.Lock()
	defer self.mu.Unlock()
	self.ctxCancel()
	self.clients.Range(func(key, value interface{}) bool {
		if v, ok := value.(*client); ok {
			v.Close()
			storage.Cache.Delete(fmt.Sprintf(storage.CacheKeyDockerStatus, v.dockerEnv.Name))
		}
		return true
	})
}

// Join 在加入时，首先检查之前是否存在，如果存在也强制退出，适用于编辑更新配置时
// TODO: dockerClient 在 listen、Close 和 Clients 之间的读写尚未同步。
func (self *monitor) Join(dockerEnv *types.DockerEnv) {
	self.mu.Lock()
	defer self.mu.Unlock()

	c := &client{
		dockerEnv: dockerEnv,
		createdAt: time.Now(),
	}
	c.ctx, c.ctxCancel = context.WithCancel(self.ctx)
	if v, ok := self.clients.Swap(dockerEnv.Name, c); ok {
		v.(*client).Close()
	}
	storage.Cache.Delete(fmt.Sprintf(storage.CacheKeyDockerStatus, dockerEnv.Name))

	go self.listen(c)
}

func (self *monitor) Leave(name string) {
	self.mu.Lock()
	defer self.mu.Unlock()
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
	status := types.DockerStatus{}
	cacheKey := fmt.Sprintf(storage.CacheKeyDockerStatus, c.dockerEnv.Name)

	for {
		if c.ctx.Err() != nil {
			return
		}
		// 同名环境更新后，旧监控不能覆盖新连接的状态。
		self.mu.Lock()
		if v, ok := self.clients.Load(c.dockerEnv.Name); !ok || v.(*client).createdAt.After(c.createdAt) {
			self.mu.Unlock()
			c.Close()
			return
		}
		if c.ctx.Err() != nil {
			self.mu.Unlock()
			return
		}
		if initErr != nil {
			status.Available = false
			status.Message = fmt.Sprintf("%s, at %s", initErr.Error(), time.Now().Format(define.DateShowYmdHis))
			storage.Cache.Set(cacheKey, status, cache.NoExpiration)
		}
		self.mu.Unlock()
		if initErr != nil {
			facade.GetEvent().Publish(event.DockerDaemonEvent, event.DockerDaemonPayload{
				DockerEnvName: c.dockerEnv.Name,
				Status:        status,
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

		// 端口可达性按实际连接链路探测，Docker API 失败不等同于端口不可达。
		reachable, err := c.checkConnection()
		self.mu.Lock()
		if v, ok := self.clients.Load(c.dockerEnv.Name); !ok || v != c || c.ctx.Err() != nil {
			self.mu.Unlock()
			return
		}
		status.Reachable = reachable
		storage.Cache.Set(cacheKey, status, cache.NoExpiration)
		self.mu.Unlock()
		if err != nil {
			initErr = err
			continue
		}

		slog.Debug("monitor start", "name", c.dockerEnv.Name, "remoteType", c.dockerEnv.RemoteType, "tlsEnabled", c.dockerEnv.EnableTLS, "previousAttemptFailed", initErr != nil)

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
		self.mu.Lock()
		if v, ok := self.clients.Load(c.dockerEnv.Name); !ok || v != c || c.ctx.Err() != nil {
			self.mu.Unlock()
			c.Clear()
			return
		}
		status.Available = true
		status.Message = ""
		storage.Cache.Set(cacheKey, status, cache.NoExpiration)
		self.mu.Unlock()

		slog.Debug("monitor publish event", "name", c.dockerEnv.Name)
		facade.GetEvent().Publish(event.DockerDaemonEvent, event.DockerDaemonPayload{
			DockerEnvName: c.dockerEnv.Name,
			Status:        status,
		})

		eventCtx, eventCancel := context.WithCancel(c.ctx)
		eventChan, errChan := c.dockerClient.Client.Events(eventCtx, events.ListOptions{})
		checkTicker := time.NewTicker(10 * time.Second)

	eventLoop:
		for {
			select {
			case <-c.ctx.Done():
				slog.Debug("monitor closed by monitor", "name", c.dockerEnv.Name)
				break eventLoop
			case <-self.ctx.Done():
				slog.Debug("monitor closed")
				break eventLoop
			case <-checkTicker.C:
				// 跳板仍在线时，事件流可能迟迟不报错，必须检查目标 Docker 的响应。
				pingCtx, pingCancel := context.WithTimeout(c.ctx, define.DockerConnectServerTimeout)
				_, initErr = c.dockerClient.Client.Ping(pingCtx)
				pingCancel()
				if initErr != nil {
					break eventLoop
				}
			case message, ok := <-eventChan:
				if !ok {
					initErr = fmt.Errorf("docker event stream closed")
					break eventLoop
				}
				if facade.GetConfig().GetString("app.env") == "debug" {
					if _, _, ok := function.PluckArrayItemWalk(skipActionLog, func(item string) bool {
						return strings.HasPrefix(string(message.Action), item)
					}); !ok {
						logMessage := message
						logMessage.Actor.Attributes = function.PluckMapWithKeys(message.Actor.Attributes, keepAttribute)
						slog.Debug("monitor message", "name", c.dockerEnv.Name, "message", logMessage)
					}
				}
				self.processor(c.dockerEnv.Name, message)
			case err, ok := <-errChan:
				if c.ctx.Err() != nil || self.ctx.Err() != nil {
					break eventLoop
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

		checkTicker.Stop()
		eventCancel()
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
