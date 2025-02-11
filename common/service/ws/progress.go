package ws

import (
	"context"
	"errors"
	"github.com/donknap/dpanel/app/common/logic"
	"github.com/gin-gonic/gin"
	"time"
)

type ProgressWrite func(p []byte) ([]byte, error)

func NewProgressPip(messageType string) ProgressPip {
	ctx, cancelFunc := context.WithCancel(context.Background())
	process := ProgressPip{
		messageType: messageType,
		ctx:         ctx,
		cancel:      cancelFunc,
	}
	if p, exists := collect.progressPip.LoadAndDelete(messageType); exists {
		p.(ProgressPip).Close()
	}
	collect.progressPip.Store(messageType, process)
	return process
}

func NewFdProgressPip(http *gin.Context, messageType string) (ProgressPip, error) {
	fd := ""
	if data, exists := http.Get("userInfo"); exists {
		userInfo := data.(logic.UserInfo)
		fd = userInfo.Fd
	} else {
		return ProgressPip{}, errors.New("fd not found")
	}
	process := NewProgressPip(messageType)
	process.fd = fd
	return process, nil
}

type ProgressPip struct {
	fd          string
	messageType string
	ctx         context.Context
	cancel      context.CancelFunc
	OnWrite     func(p string) error
}

func (self ProgressPip) Write(p []byte) (n int, err error) {
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

func (self ProgressPip) BroadcastMessage(data interface{}) {
	BroadcastMessage <- &RespMessage{
		Type:   self.messageType,
		Data:   data,
		RespAt: time.Now(),
	}
}

func (self ProgressPip) Close() {
	self.cancel()
}

func (self ProgressPip) Done() <-chan struct{} {
	return self.ctx.Done()
}
