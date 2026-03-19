package ws

import (
	"bytes"
	"encoding/json"
	"time"
)

const (
	MessageTypeEvent                 = "event"
	MessageTypeEventFd               = "event:fd"
	MessageTypeEventRefresh          = "event:refresh" // 后台主动刷新前端
	MessageTypeEventRefreshDockerEnv = "event:refresh:dockerEnv"
	MessageTypeCompose               = "compose:%s"
	MessageTypeComposeLog            = "compose:log:%s"
	MessageTypeConsole               = "/console/container/%s"
	MessageTypeConsoleHost           = "/console/host/%s"
	MessageTypeContainerLog          = "container:log:%s"
	MessageTypeContainerAllStat      = "container:stat"
	MessageTypeContainerStat         = "container:stat:%s"
	MessageTypeContainerExplorer     = "container:explorer"
	MessageTypeDiskUsage             = "stat:diskUsage"
	MessageTypeContainerBackup       = "container:backup:%d"
	MessageTypeImagePull             = "image:pull:%s"
	MessageTypeImageBuild            = "image:build:%d"
	MessageTypeImageImport           = "image:import"
	MessageTypeProgressClose         = "progress:close"
	MessageTypeDomainApply           = "domain:apply"
	MessageTypeUserPermission        = "user:permission:%s"
	MessageTypeSwarmLog              = "swarm:log:%s:%s"
	MessageTypeNginxLog              = "nginx:log"
)

type RespMessage struct {
	Fd     string      `json:"fd"`
	Type   string      `json:"type"`
	Data   interface{} `json:"data"`
	RespAt time.Time   `json:"respAt,omitempty"`
}

func (self RespMessage) ToJson() []byte {
	// 防止传递的值是指针，发广播时数据发生变生产生 panic
	// 先复制成副本再操作
	if self.Data != nil {
		switch v := self.Data.(type) {
		case json.RawMessage, string, int, int64, float64, bool:
			break
		default:
			if b, err := json.Marshal(v); err == nil {
				self.Data = json.RawMessage(b)
			}
		}
	}
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
