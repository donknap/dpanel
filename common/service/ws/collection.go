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
		clients:     make(map[string]*Client),
		progressPip: make(map[string]*ProgressPip),
	}
	go obj.Broadcast()
	return obj
}

type Collection struct {
	clients     map[string]*Client
	progressPip map[string]*ProgressPip
	ctx         context.Context
}

func (self *Collection) Join(c *Client) {
	self.clients[c.Fd] = c
}

func (self *Collection) Leave(c *Client) {
	if _, ok := self.clients[c.Fd]; ok {
		delete(self.clients, c.Fd)
	}
	if len(self.clients) == 0 {
		for key, pip := range self.progressPip {
			pip.cancel()
			delete(self.progressPip, key)
		}
	}
}

func (self *Collection) sendMessage(message *RespMessage) {
	lock.Lock()
	lock.Unlock()
	for _, client := range self.clients {
		err := client.Conn.WriteMessage(websocket.TextMessage, message.ToJson())
		if err != nil {
			slog.Error("ws broadcast error", "fd", client.Fd, "error", err.Error())
		}
	}
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
	return len(self.clients)
}
