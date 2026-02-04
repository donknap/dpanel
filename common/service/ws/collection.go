package ws

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/service/notice"
	"github.com/donknap/dpanel/common/service/plugin"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/gorilla/websocket"
)

var (
	lock             = sync.RWMutex{}
	BroadcastMessage = make(SendMessageQueue)
)

func NewCollection() *Collection {
	obj := &Collection{
		clients:     sync.Map{},
		progressPip: sync.Map{},
	}
	go obj.Broadcast()
	return obj
}

func GetCollect() *Collection {
	return collect
}

type Collection struct {
	clients     sync.Map
	progressPip sync.Map
	ctx         context.Context
}

func (self *Collection) Join(c *Client) {
	self.clients.Store(c.Fd, c)
}

func (self *Collection) Leave(c *Client) {
	if _, loaded := self.clients.LoadAndDelete(c.Fd); !loaded {
		return
	}
	_ = c.Conn.Close()

	self.progressPip.Range(func(key, value any) bool {
		if v, ok := value.(*ProgressPip); ok && !v.IsKeepAlive {
			v.CloseFd(c.Fd)
		}
		return true
	})

	// ws 断开之后检测当前用户是否全部断开连接，则把缓存中的用户数据清除掉（主动退出）
	time.AfterFunc(10*time.Second, func() {
		for key, item := range storage.Cache.Items() {
			if strings.HasPrefix(key, "user:") {
				if v, ok := item.Object.(logic.UserInfo); ok && !v.AutoLogin {
					hasConnect := false
					self.clients.Range(func(key, value any) bool {
						if c, ok := value.(*Client); ok && c.UserId == v.UserId {
							hasConnect = true
							return true
						}
						return true
					})
					if !hasConnect {
						storage.Cache.Delete(key)
						slog.Debug("ws leave delete cache userinfo", "key", key, "user", v)
					}
				}
			}
		}
	})

	// 所有客户端都退出时，销毁所有通道
	if self.Total() == 0 {
		self.progressPip.Range(func(key, value any) bool {
			if p, success := value.(*ProgressPip); success && !p.IsKeepAlive {
				p.Close()
				self.progressPip.Delete(p.messageType)
			}
			return true
		})
		// 没有任何用户时，中断 docker 的所有请求
		slog.Debug("docker client cancel")

		//docker.Sdk.CtxCancelFunc()
		//docker.Sdk.Client.Close()
		//ctx, cancelFunc := context.WithCancel(context.Background())
		//docker.Sdk.Ctx = ctx
		//docker.Sdk.CtxCancelFunc = cancelFunc
		//go logic.EventLogic{}.MonitorLoop()
		if explorer, err := plugin.NewPlugin(plugin.PluginExplorer, nil); err == nil {
			_ = explorer.Destroy()
		}
	}
}

func (self *Collection) sendMessage(message *RespMessage) {
	self.clients.Range(func(key, value any) bool {
		c, ok := value.(*Client)
		if !ok {
			return true
		}
		if message.Fd != "" && c.Fd != message.Fd {
			return true
		}
		err := c.Conn.WriteMessage(websocket.TextMessage, message.ToJson())
		if err != nil {
			slog.Debug("ws broadcast error", "fd", c.Fd, "error", err.Error())
			self.Leave(c)
		}
		return true
	})
}

func (self *Collection) Broadcast() {
	for {
		select {
		case message := <-BroadcastMessage:
			self.sendMessage(message)

		case message := <-notice.QueueNoticePushMessage:
			data := &RespMessage{
				Type: "notice",
				Data: message,
			}
			self.sendMessage(data)
		}
	}
}

func (self *Collection) Total() int {
	lock.Lock()
	lock.Unlock()
	count := 0
	self.clients.Range(func(key, value any) bool {
		count += 1
		return true
	})
	return count
}

func (self *Collection) ProgressTotal() int {
	lock.Lock()
	lock.Unlock()
	count := 0
	self.progressPip.Range(func(key, value any) bool {
		count += 1
		return true
	})
	return count
}
