package logic

import (
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/we7coreteam/w7-rangine-go-support/src/facade"
	"log/slog"
	"mime/multipart"
)

const REGISTER_NAME = "containerTask"

func RegisterDockerTask() {
	err := facade.GetContainer().NamedSingleton(
		REGISTER_NAME, func() *DockerTask {
			obj := &DockerTask{}
			obj.QueueCreate = make(chan *CreateMessage, 999)
			obj.containerStepMessage = make(map[int32]*containerStepMessage)

			obj.QueueBuildImage = make(chan *BuildImageMessage, 999)
			obj.imageStepMessage = make(map[int32]*imageStepMessage)
			return obj
		},
	)
	if err != nil {
		panic(err)
	}
}

func NewDockerTask() *DockerTask {
	var obj *DockerTask
	err := facade.GetContainer().NamedResolve(&obj, REGISTER_NAME)
	if err != nil {
		slog.Error(err.Error())
	}
	return obj
}

type CreateMessage struct {
	Name      string // 站点标识
	SiteId    int32  // 站点id
	RunParams *accessor.SiteEnvOption
}

type BuildImageMessage struct {
	ZipPath           string // 构建包
	ZipPathUploadFile multipart.File
	DockerFileContent []byte // 自定义Dockerfile
	DockerFileInPath  string // Dockerfile 所在路径
	Tag               string // 镜像Tag
	ImageId           int32
	Context           string // Dockerfile 所在的目录
}

type DockerTask struct {
	QueueCreate          chan *CreateMessage             // 用于放置构建容器任务
	QueueBuildImage      chan *BuildImageMessage         // 用于放置构建镜像任务
	containerStepMessage map[int32]*containerStepMessage // 用于记录部署任务日志
	imageStepMessage     map[int32]*imageStepMessage     // 用于记录构建镜像中日志
	sdk                  *docker.Builder
}

func (self *DockerTask) GetTaskContainerStepLog(taskId int32) *containerStepMessage {
	if stepLog, ok := self.containerStepMessage[taskId]; ok {
		return stepLog
	}
	return nil
}

func (self *DockerTask) GetTaskImageBuildStepLog(taskId int32) *imageStepMessage {
	if stepLog, ok := self.imageStepMessage[taskId]; ok {
		return stepLog
	}
	return nil
}
