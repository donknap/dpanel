package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/notice"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"log/slog"
	"net/http"
	"runtime"
	"time"
)

func NewClientConn(ctx *gin.Context) (*Client, error) {
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
		conn:          wsConn,
		CtxContext:    ctxWs,
		CtxCancelFunc: ctxWsCancel,
	}
	client.SendMessageQueue = make(chan string)
	slog.Info("ws connect", "fd", client.Id, "goroutine", runtime.NumGoroutine())
	return client, nil
}

type Client struct {
	Id               string          // ws 用户连接标识
	Fd               string          // 业务系统中用户唯一标识
	conn             *websocket.Conn // 当前 ws 连接
	CtxCancelFunc    context.CancelFunc
	CtxContext       context.Context
	SendMessageQueue chan string
}

type RecvMessage struct {
	Fd      string `json:"fd"`      // 发送消息id
	Content string `json:"content"` // 消息内容
	RecvAt  int64  `json:"recv_at"`
	Type    int    `json:"type"` // 消息类型
}

type RespMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

func (self RespMessage) ToJson() []byte {
	jsonStr, _ := json.Marshal(self)
	return jsonStr
}

func (self *Client) ReadMessage() {
	for {
		select {
		case <-self.CtxContext.Done():
			return
		default:
			recvMsgType, recvMsg, err := self.conn.ReadMessage()
			if err != nil {
				slog.Info("stop read message goroutine", "fd", self.Id)
				self.Close()
				return
			}
			message := &RecvMessage{
				Fd:      self.Fd,
				Content: string(recvMsg),
				Type:    recvMsgType,
				RecvAt:  time.Now().Unix(),
			}
			if message.Content == "ping" {
				self.SendMessageQueue <- "PONG"
				continue
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
			data := &RespMessage{
				Type: "event",
				Data: message,
			}
			self.conn.WriteMessage(websocket.TextMessage, data.ToJson())
		case message := <-notice.QueueNoticePushMessage:
			data := &RespMessage{
				Type: "notice",
				Data: message,
			}
			self.conn.WriteMessage(websocket.TextMessage, data.ToJson())
		case message := <-docker.QueueDockerProgressMessage:
			data := &RespMessage{
				Type: "imageBuild",
				Data: message,
			}
			err := self.conn.WriteMessage(websocket.TextMessage, data.ToJson())
			fmt.Printf("%v \n", err)
		}
	}
}

func (self *Client) Close() error {
	self.conn.CloseHandler()(websocket.ClosePolicyViolation, "close repeat login")
	self.conn.Close()
	self.CtxCancelFunc()
	return nil
}
