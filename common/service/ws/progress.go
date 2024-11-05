package ws

import (
	"context"
	"time"
)

type ProgressWrite func(p []byte) ([]byte, error)

func NewProgressPip(messageType string) *ProgressPip {
	ctx, cancelFunc := context.WithCancel(context.Background())
	process := &ProgressPip{
		messageType: messageType,
		ctx:         ctx,
		cancel:      cancelFunc,
	}
	if progress, ok := collect.progressPip[messageType]; ok {
		progress.cancel()
	}
	collect.progressPip[messageType] = process
	return process
}

type ProgressPip struct {
	messageType string
	ctx         context.Context
	cancel      context.CancelFunc
	OnWrite     func(p string) error
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
	if pip, ok := collect.progressPip[self.messageType]; ok {
		pip.cancel()
		delete(collect.progressPip, self.messageType)
	}
}

func (self ProgressPip) Done() <-chan struct{} {
	return self.ctx.Done()
}
