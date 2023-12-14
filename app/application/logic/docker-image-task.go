package logic

import (
	"fmt"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/notice"
)

func (self DockerTask) ImageBuild(buildImageTask *BuildImageMessage) error {
	go func() {
		builder := docker.Sdk.GetImageBuildBuilder()
		if buildImageTask.ZipPath != "" {
			builder.WithZipFilePath(buildImageTask.ZipPath)
		}
		if buildImageTask.DockerFileContent != nil {
			builder.WithDockerFileContent(buildImageTask.DockerFileContent)
		}
		if buildImageTask.Context != "" {
			builder.WithDockerFilePath(buildImageTask.Context)
		}
		if buildImageTask.GitUrl != "" {
			builder.WithGitUrl(buildImageTask.GitUrl)
		}
		builder.WithTag(buildImageTask.Tag)
		response, err := builder.Execute()
		if err != nil {
			dao.Image.Where(dao.Image.ID.Eq(buildImageTask.ImageId)).Updates(entity.Image{
				Status:  STATUS_ERROR,
				Message: err.Error(),
			})
			notice.Message{}.Error("imageBuild", err.Error())
			return
		}

		buildProgressMessage := ""

		defer response.Body.Close()
		progressChan := docker.Sdk.Progress(response.Body, fmt.Sprintf("%d", buildImageTask.ImageId))
		for {
			select {
			case message, ok := <-progressChan:
				if !ok {
					notice.Message{}.Success("imageBuild", buildImageTask.Tag)
					dao.Image.Select(dao.Image.Message, dao.Image.Status).Where(dao.Image.ID.Eq(buildImageTask.ImageId)).Updates(entity.Image{
						Status:  STATUS_SUCCESS,
						Message: "",
					})
					return
				}
				if message.Aux != nil && message.Aux.Aux.ID != "" {
					// md5
				}
				if message.Stream != nil {
					buildProgressMessage += message.Stream.Stream
					docker.QueueDockerProgressMessage <- message
				}
				if message.Err != nil {
					dao.Image.Where(dao.Image.ID.Eq(buildImageTask.ImageId)).Updates(entity.Image{
						Status:  STATUS_ERROR,
						Message: buildProgressMessage,
					})
					notice.Message{}.Error("imageBuild", message.Err.Error())
					return
				}
			}
		}
	}()
	return nil
}

//type progressStream struct {
//	Stream string `json:"stream"`
//}
//
//type progressImageBuild struct {
//	StepTotal   string   `json:"stepTotal"`
//	StepCurrent string   `json:"stepCurrent"`
//	Message     []string `json:"message"`
//	Error       string   `json:"error"`
//	Downloading float64  `json:"downloading"`
//	Extracting  float64  `json:"extracting"`
//}
//
//type progressImageBuildErr struct {
//	ErrorDetail struct {
//		Message string
//	} `json:"errorDetail"`
//	Error string `json:"error"`
//}
//
//type aux struct {
//	Aux struct {
//		ID string
//	}
//}

//func (self *DockerTask) ImageBuildLoop() {
//	self.sdk = docker.Sdk
//
//	for {
//		select {
//		case message := <-self.QueueBuildImage:
//			for key, _ := range self.imageStepMessage {
//				delete(self.imageStepMessage, key)
//			}
//
//			slog.Info(fmt.Sprintf("build image id %d", message.ImageId))
//			self.imageStepMessage[message.ImageId] = newImageStepMessage(message.ImageId)
//			self.imageStepMessage[message.ImageId].step(STEP_IMAGE_BUILD)
//
//			builder := docker.Sdk.GetImageBuildBuilder()
//			if message.ZipPath != "" {
//				builder.WithZipFilePath(message.ZipPath)
//			}
//			if message.DockerFileContent != nil {
//				builder.WithDockerFileContent(message.DockerFileContent)
//			}
//			if message.Context != "" {
//				builder.WithDockerFilePath(message.Context)
//			}
//			if message.GitUrl != "" {
//				builder.WithGitUrl(message.GitUrl)
//			}
//			builder.WithTag(message.Tag)
//			self.imageStepMessage[message.ImageId].step(STEP_IMAGE_BUILD_UPLOAD_TAR)
//			response, err := builder.Execute()
//			if err != nil {
//				slog.Error(err.Error())
//				notice.QueueNoticePushMessage <- &entity.Notice{
//					Type:    "error",
//					Title:   "image.build",
//					Message: err.Error(),
//				}
//				self.imageStepMessage[message.ImageId].err(err)
//				break
//			}
//			defer response.Body.Close()
//			self.imageStepMessage[message.ImageId].step(STEP_IMAGE_BUILD_RUN)
//			pg := progressImageBuild{
//				StepTotal:   "0",
//				StepCurrent: "0",
//				Downloading: 0,
//				Extracting:  0,
//			}
//			out := bufio.NewReader(response.Body)
//			for {
//				str, _, err := out.ReadLine()
//				if err == io.EOF {
//					break
//				} else {
//					if bytes.Contains(str, []byte("errorDetail")) {
//						stream := &progressImageBuildErr{}
//						err = json.Unmarshal(str, &stream)
//						if err != nil {
//							slog.Error(err.Error())
//							continue
//						}
//						pg.Error = stream.Error
//						slog.Error(stream.Error)
//						self.imageStepMessage[message.ImageId].err(errors.New(stream.Error))
//						break
//					} else if bytes.Contains(str, []byte("{\"aux\":")) {
//						stream := &aux{}
//						err = json.Unmarshal(str, &stream)
//						if err != nil {
//							slog.Error(err.Error())
//							continue
//						}
//						imageInfo, _, err := self.sdk.Client.ImageInspectWithRaw(context.Background(), stream.Aux.ID)
//						if err != nil {
//							self.imageStepMessage[message.ImageId].err(err)
//							continue
//						}
//						self.imageStepMessage[message.ImageId].sync(imageInfo)
//					} else {
//						stream := &progressStream{}
//						fmt.Printf("%v \n", string(str))
//						err = json.Unmarshal(str, &stream)
//						if err != nil {
//							slog.Error(err.Error())
//							continue
//						}
//						pg.Message = append(pg.Message, stream.Stream)
//
//						field := strings.Fields(stream.Stream)
//						if field != nil && len(field) > 0 && field[0] == "Step" {
//							step := strings.Split(field[1], "/")
//							pg.StepTotal = step[1]
//							pg.StepCurrent = step[0]
//						}
//						self.imageStepMessage[message.ImageId].process(pg)
//					}
//				}
//			}
//			if pg.Error == "" {
//				self.imageStepMessage[message.ImageId].success()
//			}
//		default:
//
//		}
//	}
//}
