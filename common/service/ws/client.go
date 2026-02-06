package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/donknap/dpanel/app/common/logic"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var (
	collect         = NewCollection()
	sendMessageLock = sync.RWMutex{}
)

func NewClient(ctx *gin.Context, options ...Option) (*Client, error) {
	fd := fmt.Sprintf("fd:%s", ctx.Request.Header.Get("Sec-WebSocket-Key"))
	// ws 主动关掉管道
	options = append(options, WithMessageRecvHandler(MessageTypeProgressClose, func(message *RecvMessage) {
		closeMessage := struct {
			Type string `json:"type"`
			Data string `json:"data"`
		}{}
		slog.Debug("ws event", "event", MessageTypeProgressClose, "fd", fd, "message", string(message.Message))
		if err := json.Unmarshal(message.Message, &closeMessage); err == nil {
			if p, ok := collect.progressPip.Load(closeMessage.Data); ok {
				if v, ok := p.(*ProgressPip); ok {
					v.CloseFd(fd)
				}
			}
		}
	}))
	ws := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
		Error: func(w http.ResponseWriter, r *http.Request, status int, reason error) {},
	}
	wsConn, err := ws.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		return nil, err
	}
	ctxWs, ctxWsCancel := context.WithCancel(context.Background())
	client := &Client{
		Fd:            fd,
		Conn:          wsConn,
		CtxContext:    ctxWs,
		CtxCancelFunc: ctxWsCancel,
	}
	for _, option := range options {
		err = option(client)
		if err != nil {
			return nil, err
		}
	}
	if v, ok := ctx.Get("userInfo"); ok {
		client.UserId = v.(logic.UserInfo).UserId
	}
	collect.Join(client)
	slog.Info("ws connect", "fd", client.Fd, "goroutine", runtime.NumGoroutine(), "client total", collect.Total(), "progress total", collect.ProgressTotal())
	return client, nil
}

type Client struct {
	UserId             int32
	Fd                 string          // 业务系统中用户唯一标识
	Conn               *websocket.Conn // 当前 ws 连接
	CtxCancelFunc      context.CancelFunc
	CtxContext         context.Context
	recvMessageHandler map[string]RecvMessageHandlerFn
}

func (self *Client) ReadMessage() {
	for {
		select {
		case <-self.CtxContext.Done():
			return
		default:
			recvMsgType, recvMsg, err := self.Conn.ReadMessage()
			if err != nil {
				slog.Info("stop read message goroutine", "fd", self.Fd)
				err = self.Close()
				if err != nil {
					slog.Warn("websocket", "client close", err)
				}
				return
			}
			recv := &RecvMessage{
				Fd:      self.Fd,
				Type:    recvMsgType,
				Message: recvMsg,
				RecvAt:  time.Now().Unix(),
			}
			if recv.IsPing() {
				BroadcastMessage <- &RespMessage{
					Type: MessageTypeEvent,
					Data: "pong",
				}
				continue
			}
			content := recvMessageContent{}
			err = json.Unmarshal(recv.Message, &content)
			if err != nil {
				slog.Error("websocket", "unmarshal content", err)
			}
			if handler, ok := self.recvMessageHandler[content.Type]; ok {
				handler(recv)
			}
		}
	}
}

func (self *Client) SendMessage(message *RespMessage) error {
	sendMessageLock.Lock()
	defer sendMessageLock.Unlock()
	err := self.Conn.WriteMessage(websocket.TextMessage, message.ToJson())
	if err != nil {
		return err
	}
	return nil
}

func (self *Client) Close() error {
	collect.Leave(self)
	err := self.Conn.CloseHandler()(websocket.ClosePolicyViolation, "close repeat login")
	if err != nil {
		return err
	}
	err = self.Conn.Close()
	if err != nil {
		return err
	}
	self.CtxCancelFunc()
	return nil
}
