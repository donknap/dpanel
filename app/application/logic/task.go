package logic

import (
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
)

const (
	STEP_IMAGE_PULL      = "imagePull"
	STEP_IMAGE_BUILD     = "imageBuild"
	STEP_CONTAINER_BUILD = "containerBuild"
	STEP_CONTAINER_RUN   = "containerRun"
)

func newStepMessage(taskLinkId int32) *stepMessage {
	task := &stepMessage{}
	taskRow, _ := dao.Task.Where(dao.Task.TaskLinkID.Eq(taskLinkId)).First()
	if taskRow == nil {
		taskRow = &entity.Task{
			TaskLinkID: taskLinkId,
			Status:     STATUS_STOP,
			Message:    "",
		}
		dao.Task.Create(taskRow)
	}
	task.recordId = taskRow.ID
	return task
}

type stepMessage struct {
	progress    interface{} // 进度信息 json 格式
	currentStep string      // 当前进行的步骤
	error       error       // 发生的错误
	recordId    int32
}

// 记录任务错误
func (self *stepMessage) err(err error) {
	dao.Task.Where(dao.Task.ID.Eq(self.recordId)).Updates(entity.Task{
		Status:  STATUS_ERROR,
		Message: err.Error(),
	})
}

// 更新任务进度
func (self *stepMessage) step(step string) {
	dao.Task.Where(dao.Task.ID.Eq(self.recordId)).Updates(entity.Task{
		Status:  STATUS_PROCESSING,
		Message: step,
	})
}

func (self *stepMessage) process(data interface{}) {
	self.progress = data
}

func (self *stepMessage) GetProcess() interface{} {
	return self.progress
}

func (self *stepMessage) success() {
	dao.Task.Where(dao.Task.ID.Eq(self.recordId)).Updates(entity.Task{
		Status:  STATUS_SUCCESS,
		Message: "",
	})
}
