package logic

import (
	"fmt"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
)

func newStepMessage(siteId int32) *stepMessage {
	// 清除掉当前站点之前的任务
	// dao.Task.Where(dao.Task.SiteID.Eq(siteId)).Delete()

	task := &stepMessage{}
	taskRow := &entity.Task{
		TaskID:  siteId,
		Status:  STATUS_STOP,
		Message: "",
	}
	dao.Task.Create(taskRow)
	task.recordId = taskRow.ID
	task.siteId = siteId
	return task
}

type stepMessage struct {
	progress    interface{} // 进度信息 json 格式
	currentStep string      // 当前进行的步骤
	error       error       // 发生的错误
	recordId    int32
	siteId      int32
}

// 记录任务错误
func (self *stepMessage) err(err error) {
	dao.Task.Where(dao.Task.ID.Eq(self.recordId)).Updates(
		entity.Task{
			Status:  STATUS_ERROR,
			Message: err.Error(),
		},
	)
	//状态同步到站点上
	self.syncSiteStatus(STATUS_ERROR)
}

// 更新任务进度
func (self *stepMessage) step(step string) {
	dao.Task.Where(dao.Task.ID.Eq(self.recordId)).Updates(
		entity.Task{
			Status: STATUS_PROCESSING,
			Step:   step,
		},
	)
	//状态同步到站点上
	self.syncSiteStatus(STATUS_PROCESSING)
}

func (self *stepMessage) process(data interface{}) {
	self.progress = data
}

func (self *stepMessage) GetProcess() interface{} {
	return self.progress
}

func (self *stepMessage) success() {
	query := dao.Task.Where(dao.Task.ID.Eq(self.recordId))
	query.Updates(entity.Task{
		Status:  STATUS_SUCCESS,
		Message: "",
	})
	//状态同步到站点上
	self.syncSiteStatus(STATUS_SUCCESS)
}

func (self *stepMessage) syncSiteStatus(status int) {
	_, err := dao.Site.Where(dao.Site.ID.Eq(self.siteId)).Update(dao.Site.Status, status)
	if err != nil {
		fmt.Printf("%s", err.Error())
	}
}

func (self *stepMessage) syncSiteContainerInfo(containerInfoId string) {
	_, err := dao.Site.Where(dao.Site.ID.Eq(self.siteId)).Updates(&entity.Site{
		ContainerInfo: &accessor.SiteContainerInfoOption{
			ID: containerInfoId,
		},
	})
	if err != nil {
		fmt.Printf("%s", err.Error())
	}
}
