package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/ws"
	"github.com/gorilla/websocket"
)

type WsMessage struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

func (self *Client) ConnectWs(onMessage func(WsMessage)) error {
	if self.wsConn != nil {
		return errors.New("CLI websocket is already connected")
	}
	uri, err := url.JoinPath(self.apiUrl, function.RouterRootWs(), "common/notice")
	if err != nil {
		return err
	}
	wsUrl, err := url.Parse(uri)
	if err != nil {
		return err
	}
	wsUrl.Scheme = "ws"
	token, err := self.token()
	if err != nil {
		return err
	}
	wsConn, response, err := (&websocket.Dialer{HandshakeTimeout: 10 * time.Second}).Dial(wsUrl.String(), http.Header{
		"Authorization": []string{"Bearer " + token},
	})
	if err != nil {
		if response != nil && response.Body != nil {
			_ = response.Body.Close()
		}
		return fmt.Errorf("connect CLI websocket: %w", err)
	}
	if err = wsConn.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
		_ = wsConn.Close()
		return err
	}
	for {
		message := WsMessage{}
		if err = wsConn.ReadJSON(&message); err != nil {
			_ = wsConn.Close()
			return fmt.Errorf("receive CLI websocket fd: %w", err)
		}
		if message.Type != ws.MessageTypeEventFd {
			continue
		}
		if err = json.Unmarshal(message.Data, &self.fd); err != nil || self.fd == "" {
			_ = wsConn.Close()
			return errors.New("CLI websocket fd is invalid")
		}
		break
	}
	if err = wsConn.SetReadDeadline(time.Time{}); err != nil {
		_ = wsConn.Close()
		return err
	}
	self.ctx, self.wsCancel = context.WithCancelCause(self.ctx)
	self.wsConn = wsConn
	self.wsDone = make(chan struct{})
	go func() {
		defer close(self.wsDone)
		for {
			message := WsMessage{}
			if err := wsConn.ReadJSON(&message); err != nil {
				self.wsCancel(fmt.Errorf("CLI websocket disconnected: %w", err))
				return
			}
			if onMessage != nil {
				onMessage(message)
			}
		}
	}()
	return nil
}

func (self *Client) Close() {
	if self.wsConn == nil {
		return
	}
	_ = self.wsConn.Close()
	<-self.wsDone
	self.fd = ""
	self.wsConn = nil
}
