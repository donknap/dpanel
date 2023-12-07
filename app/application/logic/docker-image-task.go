package logic

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/donknap/dpanel/common/service/docker"
	"io"
	"log/slog"
	"strings"
)

type progressStream struct {
	Stream string `json:"stream"`
}

type progressImageBuild struct {
	StepTotal   string   `json:"stepTotal"`
	StepCurrent string   `json:"stepCurrent"`
	Message     []string `json:"message"`
	Error       string   `json:"error"`
	Downloading float64  `json:"downloading"`
	Extracting  float64  `json:"extracting"`
}

type progressImageBuildErr struct {
	ErrorDetail struct {
		Message string
	} `json:"errorDetail"`
	Error string `json:"error"`
}

type aux struct {
	Aux struct {
		ID string
	}
}

func (self *DockerTask) ImageBuildLoop() {
	sdk, err := docker.NewDockerClient()
	if err != nil {
		panic(err)
	}
	self.sdk = sdk

	for {
		select {
		case message := <-self.QueueBuildImage:
			for key, _ := range self.imageStepMessage {
				delete(self.imageStepMessage, key)
			}

			slog.Info(fmt.Sprintf("build image id %d", message.ImageId))
			self.imageStepMessage[message.ImageId] = newImageStepMessage(message.ImageId)
			self.imageStepMessage[message.ImageId].step(STEP_IMAGE_BUILD)

			builder := sdk.GetImageBuildBuilder()
			if message.ZipPath != "" {
				builder.WithZipFilePath(message.ZipPath)
			}
			if message.DockerFileContent != nil {
				builder.WithDockerFileContent(message.DockerFileContent)
			}
			builder.WithTag(message.Name)
			self.imageStepMessage[message.ImageId].step(STEP_IMAGE_BUILD_UPLOAD_TAR)
			response, err := builder.Execute()
			if err != nil {
				slog.Error(err.Error())
				self.imageStepMessage[message.ImageId].err(err)
				break
			}
			defer response.Body.Close()
			self.imageStepMessage[message.ImageId].step(STEP_IMAGE_BUILD_RUN)
			pg := progressImageBuild{
				StepTotal:   "0",
				StepCurrent: "0",
				Downloading: 0,
				Extracting:  0,
			}
			out := bufio.NewReader(response.Body)
			for {
				str, _, err := out.ReadLine()
				if err == io.EOF {
					break
				} else {
					if bytes.Contains(str, []byte("errorDetail")) {
						stream := &progressImageBuildErr{}
						err = json.Unmarshal(str, &stream)
						if err != nil {
							slog.Error(err.Error())
							continue
						}
						pg.Error = stream.Error
						slog.Error(stream.Error)
						self.imageStepMessage[message.ImageId].err(errors.New(stream.Error))
						break
					} else if bytes.Contains(str, []byte("{\"aux\":")) {
						stream := &aux{}
						err = json.Unmarshal(str, &stream)
						if err != nil {
							slog.Error(err.Error())
							continue
						}
						self.imageStepMessage[message.ImageId].sync(stream.Aux.ID)
					} else {
						stream := &progressStream{}
						fmt.Printf("%v \n", string(str))
						err = json.Unmarshal(str, &stream)
						if err != nil {
							slog.Error(err.Error())
							continue
						}
						pg.Message = append(pg.Message, stream.Stream)

						field := strings.Fields(stream.Stream)
						if field != nil && len(field) > 0 && field[0] == "Step" {
							step := strings.Split(field[1], "/")
							pg.StepTotal = step[1]
							pg.StepCurrent = step[0]
						}
						self.imageStepMessage[message.ImageId].process(pg)
					}
				}
			}
			if pg.Error == "" {
				self.imageStepMessage[message.ImageId].success()
			}
		default:

		}
	}
}
