package notice

import (
	"encoding/json"
	"fmt"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"runtime"
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
	jsonMessage, _ := json.Marshal(message)
	row := &entity.Notice{
		Title:     title,
		Message:   string(jsonMessage),
		Type:      level,
		CreatedAt: time.Now().Local(),
	}
	err := dao.Notice.Create(row)
	fmt.Printf("协程数，%v \n", runtime.NumGoroutine())
	QueueNoticePushMessage <- row
	return err
}

func (self Message) New(title string, message ...string) error {
	return function.ErrorMessage(title, message...)
}
