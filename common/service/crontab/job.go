package crontab

import (
	"fmt"
	"github.com/donknap/dpanel/common/dao"
)

type Job struct {
	Id            int32
	Script        string
	ContainerName string
}

func (self Job) Run() {
	fmt.Printf("%v \n", self.Script)
	fmt.Printf("%v \n", self.Id)
	self.updateNextRunTime()
}

func (self Job) updateNextRunTime() {
	if task, err := dao.Cron.Where(dao.Cron.ID.Eq(self.Id)).First(); err == nil {
		task.Setting.NextRunTime = Wrapper.GetNextRunTime(task.Setting.JobIds...)
		_, _ = dao.Cron.Updates(task)
	}
}
