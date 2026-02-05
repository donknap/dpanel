package ws

import (
	"context"
	"errors"
	"time"

	"github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/function"
	"github.com/gin-gonic/gin"
)

type ProgressWrite func(p []byte) ([]byte, error)

func NewProgressPip(messageType string) *ProgressPip {
	ctx, cancelFunc := context.WithCancel(context.Background())
	process := &ProgressPip{
		messageType: messageType,
		ctx:         ctx,
		cancel:      cancelFunc,
		fd:          make([]string, 0),
	}
	if p, exists := collect.progressPip.LoadAndDelete(messageType); exists {
		if v, ok := p.(*ProgressPip); ok {
			v.Close()
		}
	}
	collect.progressPip.Store(messageType, process)
	go func() {
		<-process.ctx.Done()
		collect.progressPip.Delete(process.messageType)
	}()
	return process
}

// 利用 fd 新建一个公共推送管道，多个 fd 共用一个，直到所有 fd 都退出
func NewFdProgressPip(http *gin.Context, messageType string) (*ProgressPip, error) {
	fd := ""
	if data, exists := http.Get("userInfo"); exists {
		userInfo := data.(logic.UserInfo)
		fd = userInfo.Fd
	} else {
		return nil, errors.New("fd not found")
	}
	var process *ProgressPip
	if p, ok := collect.progressPip.Load(messageType); ok {
		// 当管道的上下文已经关闭过了，就不能再次使用，需要重新创建
		if v, ok := p.(*ProgressPip); ok && v.ctx.Err() == nil {
			if !function.InArray(v.fd, fd) {
				v.fd = append(v.fd, fd)
			}
			process = v
		}
	}
	if process == nil {
		process = NewProgressPip(messageType)
		process.fd = append(process.fd, fd)
	}
	return process, nil
}

type ProgressPip struct {
	fd           []string
	messageType  string
	ctx          context.Context
	cancel       context.CancelFunc
	OnWrite      func(p string) error
	OnWriteBytes func(p []byte) error
	IsKeepAlive  bool // 保持运行，除非前端终止或是 process:close，不受 ws 连接断开影响
}

func (self *ProgressPip) Write(p []byte) (n int, err error) {
	temp := string(p)
	if self.OnWrite != nil {
		err = self.OnWrite(temp)
		if err != nil {
			return 0, err
		}
	} else {
		self.BroadcastMessage(temp)
	}
	return len(p), nil
}

func (self *ProgressPip) BroadcastMessage(data interface{}) {
	BroadcastMessage <- &RespMessage{
		Type:   self.messageType,
		Data:   data,
		RespAt: time.Now(),
	}
}

func (self *ProgressPip) Close() {
	self.cancel()
}

func (self *ProgressPip) CloseFd(fd string) {
	self.fd = function.PluckArrayWalk(self.fd, func(i string) (string, bool) {
		if i != fd {
			return i, true
		}
		return "", false
	})
	if len(self.fd) == 0 {
		self.Close()
	}
}

func (self *ProgressPip) IsShadow() bool {
	return len(self.fd) > 1
}

func (self *ProgressPip) Done() <-chan struct{} {
	return self.ctx.Done()
}

func (self *ProgressPip) Context() context.Context {
	return self.ctx
}

func (self *ProgressPip) KeepAlive() *ProgressPip {
	self.IsKeepAlive = true
	return self
}
