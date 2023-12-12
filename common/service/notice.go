package service

import (
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"time"
)

const (
	NOTICE_TYPE_ERROR = "error"
	NOTICE_TYPE_INFO  = "info"
)

type Notice struct {
}

func (self Notice) Error(title string, message string) error {
	return self.newNotice(NOTICE_TYPE_ERROR, title, message)
}

func (self Notice) Info(title string, message string) error {
	return self.newNotice(NOTICE_TYPE_INFO, title, message)
}

func (self Notice) newNotice(level string, title string, message string) error {
	err := dao.Notice.Create(&entity.Notice{
		Title:     title,
		Message:   message,
		Type:      level,
		CreatedAt: time.Now().Local(),
	})
	return err
}
