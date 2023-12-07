package logic

import (
	"fmt"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
)

func newStepMessage(siteId int32) *stepMessage {
	task := &stepMessage{}
	task.SiteId = siteId
	return task
}

type stepMessage struct {
	progress    interface{} // 进度信息 json 格式
	currentStep string      // 当前进行的步骤
	error       error       // 发生的错误
	SiteId      int32       // 信息回存的 site id
}

// 记录任务错误
func (self *stepMessage) err(err error) {
	dao.Site.Where(dao.Site.ID.Eq(self.SiteId)).Updates(
		entity.Site{
			Status:  STATUS_ERROR,
			Message: err.Error(),
		},
	)
}

// 更新任务进度
func (self *stepMessage) step(step string) {
	dao.Site.Where(dao.Site.ID.Eq(self.SiteId)).Updates(
		entity.Site{
			Status:     STATUS_PROCESSING,
			StatusStep: step,
		},
	)
}

func (self *stepMessage) process(data interface{}) {
	self.progress = data
}

func (self *stepMessage) GetProcess() interface{} {
	return self.progress
}

func (self *stepMessage) success() {
	query := dao.Site.Where(dao.Site.ID.Eq(self.SiteId))
	query.Updates(entity.Site{
		Status:  STATUS_SUCCESS,
		Message: "",
	})
}

func (self *stepMessage) syncSiteContainerInfo(containerInfoId string) {
	_, err := dao.Site.Where(dao.Site.ID.Eq(self.SiteId)).Updates(&entity.Site{
		ContainerInfo: &accessor.SiteContainerInfoOption{
			ID: containerInfoId,
		},
	})
	if err != nil {
		fmt.Printf("%s", err.Error())
	}
}
