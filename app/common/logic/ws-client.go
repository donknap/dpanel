package logic

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/notice"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"log/slog"
	"net/http"
	"runtime"
	"sync"
	"time"
)

var (
	wsCollect = make(map[string]*Client)
	lock      = sync.RWMutex{}
)

type ClientOptions struct {
	CloseHandler   func()
	MessageHandler map[string]func(message []byte)
}

func NewClientConn(ctx *gin.Context, options *ClientOptions) (*Client, error) {
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
		Id:            ctx.Request.Header.Get("Sec-WebSocket-Key"),
		Conn:          wsConn,
		CtxContext:    ctxWs,
		CtxCancelFunc: ctxWsCancel,
		closeHandler:  options.CloseHandler,
	}
	client.SendMessageQueue = make(chan []byte)
	client.readMessageHandler = options.MessageHandler

	wsCollect[client.Id] = client

	slog.Info("ws connect", "fd", client.Id, "goroutine", runtime.NumGoroutine(), "total", len(wsCollect))
	return client, nil
}

type Client struct {
	Id                 string          // ws 用户连接标识
	Fd                 string          // 业务系统中用户唯一标识
	Conn               *websocket.Conn // 当前 ws 连接
	CtxCancelFunc      context.CancelFunc
	CtxContext         context.Context
	SendMessageQueue   chan []byte
	readMessageHandler map[string]func(message []byte)
	closeHandler       func()
}

type recvMessage struct {
	Fd         string `json:"fd"` // 发送消息id
	ContentStr []byte `json:"content_str"`
	RecvAt     int64  `json:"recv_at"`
	Type       int    `json:"type"` // 消息类型
}

type recvMessageContent struct {
	Type    string
	Content map[string]interface{}
}

type respMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

func (self respMessage) ToJson() []byte {
	jsonStr, _ := json.Marshal(self)
	return jsonStr
}

func (self *Client) ReadMessage() {
	for {
		select {
		case <-self.CtxContext.Done():
			return
		default:
			recvMsgType, recvMsg, err := self.Conn.ReadMessage()
			if err != nil {
				slog.Info("stop read message goroutine", "fd", self.Id)
				self.Close()
				return
			}
			message := &recvMessage{
				Fd:         self.Fd,
				ContentStr: recvMsg,
				Type:       recvMsgType,
				RecvAt:     time.Now().Unix(),
			}
			if bytes.Equal(message.ContentStr, []byte("ping")) {
				self.SendMessageQueue <- []byte("ping")
				continue
			}
			content := recvMessageContent{}
			json.Unmarshal(message.ContentStr, &content)
			if handler, ok := self.readMessageHandler[content.Type]; ok {
				handler(message.ContentStr)
			}
		}
	}
}

func (self *Client) SendMessage() {
	for {
		select {
		case <-self.CtxContext.Done():
			slog.Info("stop send message goroutine", "fd", self.Id)
			return
		case message := <-self.SendMessageQueue:
			data := &respMessage{
				Type: "event",
				Data: message,
			}
			self.sendMessage(data)
		case message := <-notice.QueueNoticePushMessage:
			data := &respMessage{
				Type: "notice",
				Data: message,
			}
			self.sendMessage(data)
		case message := <-docker.QueueDockerProgressMessage:
			data := &respMessage{
				Type: "imageBuild",
				Data: message,
			}
			self.sendMessage(data)
		case message := <-docker.QueueDockerImageDownloadMessage:
			data := &respMessage{
				Type: "imageDownload",
				Data: message,
			}
			self.sendMessage(data)
		}
	}
}

/* 抢到chan里消息的ws发送消息时都给所有客户端发送 */
func (self *Client) sendMessage(message *respMessage) {
	lock.Lock()
	for _, client := range wsCollect {
		err := client.Conn.WriteMessage(websocket.TextMessage, message.ToJson())
		if err != nil {
			slog.Error("ws send message error", "fd", self.Id, "error", err.Error())
		}
	}
	lock.Unlock()
}

func (self *Client) Close() error {
	if self.closeHandler != nil {
		self.closeHandler()
	}
	if _, ok := wsCollect[self.Id]; ok {
		delete(wsCollect, self.Id)
	}
	self.Conn.CloseHandler()(websocket.ClosePolicyViolation, "close repeat login")
	self.Conn.Close()
	self.CtxCancelFunc()
	return nil
}
