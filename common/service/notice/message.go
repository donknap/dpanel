package notice

import (
	"fmt"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"runtime"
	"strings"
	"time"
)

var (
	QueueNoticePushMessage = make(chan *entity.Notice)
)

const (
	TypeError   = "error"
	TypeInfo    = "info"
	TypeSuccess = "success"
)

type Message struct {
}

func (self Message) Error(title string, message ...string) error {
	return self.push(TypeError, title, message)
}

func (self Message) Info(title string, message ...string) error {
	return self.push(TypeInfo, title, message)
}

func (self Message) Success(title string, message ...string) error {
	return self.push(TypeSuccess, title, message)
}

func (self Message) push(level string, title string, message []string) error {
	row := &entity.Notice{
		Title:     title,
		Message:   strings.Join(message, " "),
		Type:      level,
		CreatedAt: time.Now().Local(),
	}
	err := dao.Notice.Create(row)
	fmt.Printf("协程数，%v \n", runtime.NumGoroutine())
	QueueNoticePushMessage <- row
	return err
}
