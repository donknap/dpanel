package ws

import (
	"context"
	"github.com/donknap/dpanel/common/service/notice"
	"github.com/gorilla/websocket"
	"log/slog"
	"sync"
)

var (
	lock             = sync.RWMutex{}
	BroadcastMessage = make(SendMessageQueue)
)

func NewCollection() *Collection {
	obj := &Collection{
		clients:     sync.Map{},
		progressPip: make(map[string]*ProgressPip),
	}
	go obj.Broadcast()
	return obj
}

type Collection struct {
	clients     sync.Map
	progressPip map[string]*ProgressPip
	ctx         context.Context
}

func (self *Collection) Join(c *Client) {
	self.clients.Store(c.Fd, c)
}

func (self *Collection) Leave(c *Client) {
	self.clients.Delete(c.Fd)
	if self.Total() == 0 {
		for key, pip := range self.progressPip {
			pip.cancel()
			delete(self.progressPip, key)
		}
	}
}

func (self *Collection) sendMessage(message *RespMessage) {
	lock.Lock()
	lock.Unlock()
	self.clients.Range(func(key, value any) bool {
		c := value.(*Client)
		err := c.Conn.WriteMessage(websocket.TextMessage, message.ToJson())
		if err != nil {
			slog.Error("ws broadcast error", "fd", c.Fd, "error", err.Error())
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
