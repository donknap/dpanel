package ws

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/function"
	"github.com/gin-gonic/gin"
)

type ProgressWrite func(p []byte) ([]byte, error)

func PushEvent(messageType string, data interface{}) {
	BroadcastMessage <- NewRespMessage("", messageType, data)
}

func NewProgressPip(messageType string) *ProgressPip {
	collect.progressMu.Lock()
	defer collect.progressMu.Unlock()

	return newProgressPip(progressNamespace{messageType: messageType})
}

// newProgressPip 创建并登记管道，调用方必须持有 collect.progressMu。
func newProgressPip(namespace progressNamespace) *ProgressPip {
	ctx, cancelFunc := context.WithCancel(context.Background())
	process := &ProgressPip{
		namespace:  namespace,
		progressMu: &collect.progressMu,
		ctx:        ctx,
		cancel:     cancelFunc,
		fd:         make([]string, 0),
	}
	if p, exists := collect.progressPip.LoadAndDelete(namespace); exists {
		if v, ok := p.(*ProgressPip); ok {
			v.close()
		}
	}
	collect.progressPip.Store(namespace, process)
	go func() {
		<-process.ctx.Done()
		collect.progressMu.Lock()
		defer collect.progressMu.Unlock()
		collect.progressPip.CompareAndDelete(process.namespace, process)
	}()
	return process
}

type progressNamespace struct {
	userID int32
	// Docker Client 按用户与 DockerEnv 关联后，dockerEnvName 再参与命名空间隔离。
	dockerEnvName string
	messageType   string
}

// NewFdProgressPip 同一用户、同一 Docker 环境的多个 fd 共用一个推送管道，直到所有 fd 都退出。
// owner 仅在本次请求创建管道时为 true，调用方只能由 owner 启动生产任务。
func NewFdProgressPip(http *gin.Context, dockerEnvName, messageType string) (*ProgressPip, bool, error) {
	fd := ""
	namespace := progressNamespace{
		dockerEnvName: dockerEnvName,
		messageType:   messageType,
	}
	if data, exists := http.Get("userInfo"); exists {
		userInfo := data.(logic.UserInfo)
		fd = userInfo.Fd
		namespace.userID = userInfo.UserId
	} else {
		return nil, false, errors.New("fd not found")
	}
	if fd == "" {
		return nil, false, errors.New("fd not found")
	}

	collect.progressMu.Lock()
	defer collect.progressMu.Unlock()

	var process *ProgressPip
	if p, ok := collect.progressPip.Load(namespace); ok {
		// 当管道的上下文已经关闭过了，就不能再次使用，需要重新创建
		if v, ok := p.(*ProgressPip); ok && v.ctx.Err() == nil {
			v.addFd(fd)
			return v, false, nil
		}
	}
	process = newProgressPip(namespace)
	process.addFd(fd)
	return process, true, nil
}

type ProgressPip struct {
	namespace    progressNamespace
	progressMu   *sync.Mutex
	fdLock       sync.RWMutex
	fd           []string
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
	self.fdLock.RLock()
	fds := append([]string(nil), self.fd...)
	self.fdLock.RUnlock()
	if len(fds) == 0 {
		BroadcastMessage <- NewRespMessage("", self.namespace.messageType, data)
		return
	}
	for _, fd := range fds {
		BroadcastMessage <- NewRespMessage(fd, self.namespace.messageType, data)
	}
}

func (self *ProgressPip) Close() {
	self.progressMu.Lock()
	defer self.progressMu.Unlock()

	self.close()
}

// close 关闭管道，调用方必须持有 progressMu。
func (self *ProgressPip) close() {
	self.cancel()
}

func (self *ProgressPip) CloseFd(fd string) {
	self.progressMu.Lock()
	defer self.progressMu.Unlock()

	self.fdLock.Lock()
	total := len(self.fd)
	self.fd = function.PluckArrayWalk(self.fd, func(i string) (string, bool) {
		if i != fd {
			return i, true
		}
		return "", false
	})
	empty := total > len(self.fd) && len(self.fd) == 0
	self.fdLock.Unlock()
	if empty {
		self.close()
	}
}

func (self *ProgressPip) addFd(fd string) {
	self.fdLock.Lock()
	defer self.fdLock.Unlock()
	if !function.InArray(self.fd, fd) {
		self.fd = append(self.fd, fd)
	}
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

func (self *ProgressPip) String() string {
	self.fdLock.RLock()
	defer self.fdLock.RUnlock()
	return fmt.Sprintf("messageType: %s, fd: %s", self.namespace.messageType, strings.Join(self.fd, ","))
}
