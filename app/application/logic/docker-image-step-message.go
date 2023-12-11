package logic

import (
	"github.com/docker/docker/api/types"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
)

type imageStepMessage struct {
	progress    interface{} // 进度信息 json 格式
	currentStep string      // 当前进行的步骤
	error       error       // 发生的错误
	ImageId     int32       // 信息回存的 site id
}

func newImageStepMessage(id int32) *imageStepMessage {
	task := &imageStepMessage{}
	task.ImageId = id
	return task
}

// 记录任务错误
func (self *imageStepMessage) err(err error) {
	dao.Image.Where(dao.Image.ID.Eq(self.ImageId)).Updates(
		entity.Image{
			Status:  STATUS_ERROR,
			Message: err.Error(),
		},
	)
}

// 更新任务进度
func (self *imageStepMessage) step(step string) {
	dao.Image.Where(dao.Image.ID.Eq(self.ImageId)).Updates(
		entity.Image{
			Status:     STATUS_PROCESSING,
			StatusStep: step,
		},
	)
}

func (self *imageStepMessage) process(data interface{}) {
	self.progress = data
}

func (self *imageStepMessage) GetProcess() interface{} {
	return self.progress
}

func (self *imageStepMessage) success() {
	query := dao.Image.Where(dao.Image.ID.Eq(self.ImageId))
	query.Updates(entity.Image{
		Status:     STATUS_SUCCESS,
		Message:    "",
		StatusStep: "success",
	})
}

func (self *imageStepMessage) sync(info types.ImageInspect) {
	oldImageRow, _ := dao.Image.Where(dao.Image.Md5.Eq(info.ID)).First()
	if oldImageRow != nil {
		dao.Image.Where(dao.Image.ID.Eq(oldImageRow.ID)).Delete()
	}
	dao.Image.Where(dao.Image.ID.Eq(self.ImageId)).Updates(&entity.Image{
		Md5: info.ID,
	})
}
