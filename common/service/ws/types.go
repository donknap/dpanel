package ws

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"
)

const (
	MessageTypeEvent                  = "event"
	MessageTypeEventFd                = "event:fd"
	MessageTypeEventRefresh           = "event:refresh" // 后台主动刷新前端
	MessageTypeEventRefreshDockerEnv  = "event:refresh:dockerEnv"
	MessageTypeCompose                = "compose:%s"
	MessageTypeComposeLog             = "compose:log:%s"
	MessageTypeConsole                = "/console/container/%s"
	MessageTypeConsoleSsh             = "/console/ssh/%s"
	MessageTypeConsoleShell           = "/console/shell"
	MessageTypeContainerLog           = "container:log:%s"
	MessageTypeContainerCommandCreate = "container:command:create:%s"
	MessageTypeContainerAllStat       = "container:stat"
	MessageTypeContainerStat          = "container:stat:%s"
	MessageTypeContainerExplorer      = "container:explorer"
	MessageTypeDiskUsage              = "stat:diskUsage"
	MessageTypeContainerBackup        = "container:backup:%d"
	MessageTypeImagePull              = "image:pull:%s"
	MessageTypeImageBuild             = "image:build:%d"
	MessageTypeImageImport            = "image:import"
	MessageTypeProgressClose          = "progress:close"
	MessageTypeDomainApply            = "domain:apply"
	MessageTypeUserPermission         = "user:permission:%s"
	MessageTypeSwarmLog               = "swarm:log:%s:%s"
	MessageTypeNginxLog               = "nginx:log"
)

type RespMessage struct {
	Fd     string      `json:"fd"`
	Type   string      `json:"type"`
	Data   interface{} `json:"data"`
	RespAt time.Time   `json:"respAt,omitempty"`
}

func NewRespMessage(fd, messageType string, data interface{}) (message *RespMessage) {
	message = &RespMessage{
		Fd:     fd,
		Type:   messageType,
		Data:   data,
		RespAt: time.Now(),
	}
	if message.Data == nil {
		return message
	}

	defer func() {
		if r := recover(); r != nil {
			slog.Warn("ws snapshot message panic", "type", message.Type, "error", fmt.Sprint(r))
			message.Data = fmt.Sprintf("snapshot ws message failed: %v", r)
		}
	}()

	switch v := message.Data.(type) {
	case json.RawMessage:
		message.Data = append(json.RawMessage(nil), v...)
	case string, int, int64, float64, bool:
	default:
		if b, err := json.Marshal(v); err == nil {
			message.Data = json.RawMessage(b)
		} else {
			slog.Warn("ws snapshot message", "type", message.Type, "error", err.Error())
			message.Data = err.Error()
		}
	}
	return message
}

func (self RespMessage) ToJson() []byte {
	jsonStr, _ := json.Marshal(self)
	return jsonStr
}

type RecvMessage struct {
	Fd      string `json:"fd"` // 发送消息id
	Message []byte `json:"message"`
	RecvAt  int64  `json:"recv_at"`
	Type    int    `json:"type"` // 消息类型
}

func (self RecvMessage) IsPing() bool {
	if bytes.Equal(self.Message, []byte("ping")) || bytes.Equal(self.Message, []byte("pong")) {
		return true
	} else {
		return false
	}
}

type recvMessageContent struct {
	Type string
}

type RecvMessageHandlerFn func(message *RecvMessage)

type SendMessageQueue chan *RespMessage // 普通队列，有数据即推送客户端

type Option func(self *Client) error
