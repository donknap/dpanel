package ws

import (
	"bytes"
	"encoding/json"
	"time"
)

const (
	MessageTypeEvent            = "event"
	MessageTypeEventFd          = "event:fd"
	MessageTypeCompose          = "compose:%s"
	MessageTypeComposeLog       = "compose:log:%s"
	MessageTypeConsole          = "/console/container/%s"
	MessageTypeConsoleHost      = "/console/host/%s"
	MessageTypeContainerLog     = "container:log:%s"
	MessageTypeContainerAllStat = "container:stat"
	MessageTypeContainerStat    = "container:stat:%s"
	MessageTypeDiskUsage        = "stat:diskUsage"
	MessageTypeContainerBackup  = "container:backup:%d"
	MessageTypeImagePull        = "image:pull:%s"
	MessageTypeImageBuild       = "image:build:%d"
	MessageTypeImageImport      = "image:import"
	MessageTypeProgressClose    = "progress:close"
	MessageTypeDomainApply      = "domain:apply"
	MessageTypeUserPermission   = "user:permission:%s"
	MessageTypeSwarmLog         = "swarm:log:%s:%s"
	MessageTypeNginxLog         = "nginx:log"
)

type RespMessage struct {
	Fd     string      `json:"fd"`
	Type   string      `json:"type"`
	Data   interface{} `json:"data"`
	RespAt time.Time   `json:"respAt,omitempty"`
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
