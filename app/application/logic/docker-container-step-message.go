package logic

import (
	"fmt"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
)

func newContainerStepMessage(siteId int32) *containerStepMessage {
	task := &containerStepMessage{}
	task.SiteId = siteId
	return task
}

type containerStepMessage struct {
	progress    interface{} // 进度信息 json 格式
	currentStep string      // 当前进行的步骤
	error       error       // 发生的错误
	SiteId      int32       // 信息回存的 site id
}

// 记录任务错误
func (self *containerStepMessage) err(err error) {
	dao.Site.Where(dao.Site.ID.Eq(self.SiteId)).Updates(
		entity.Site{
			Status:  STATUS_ERROR,
			Message: err.Error(),
		},
	)
}

// 更新任务进度
func (self *containerStepMessage) step(step string) {
	dao.Site.Where(dao.Site.ID.Eq(self.SiteId)).Updates(
		entity.Site{
			Status:     STATUS_PROCESSING,
			StatusStep: step,
		},
	)
}

func (self *containerStepMessage) process(data interface{}) {
	self.progress = data
}

func (self *containerStepMessage) GetProcess() interface{} {
	return self.progress
}

func (self *containerStepMessage) success() {
	query := dao.Site.Where(dao.Site.ID.Eq(self.SiteId))
	query.Updates(entity.Site{
		Status:  STATUS_SUCCESS,
		Message: "",
	})
}

func (self *containerStepMessage) syncSiteContainerInfo(containerInfoId string) {
	_, err := dao.Site.Where(dao.Site.ID.Eq(self.SiteId)).Updates(&entity.Site{
		ContainerInfo: &accessor.SiteContainerInfoOption{
			ID: containerInfoId,
		},
	})
	if err != nil {
		fmt.Printf("%s", err.Error())
	}
}
