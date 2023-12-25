package tests

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/registry"
	"github.com/donknap/dpanel/common/service/docker"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestContainerRemove(t *testing.T) {
	sdk, _ := docker.NewDockerClient()
	err := sdk.Client.ContainerStop(context.Background(), "phpmyadmin", container.StopOptions{})
	err = sdk.Client.ContainerRemove(context.Background(), "phpmyadmin", types.ContainerRemoveOptions{})
	fmt.Printf("%v \n", err)

}

type progressDetail struct {
	Id             string `json:"id"`
	Status         string `json:"status"`
	ProgressDetail struct {
		Current float64 `json:"current"`
		Total   float64 `json:"total"`
	} `json:"progressDetail"`
}

type pullImageProgress struct {
	Downloading float64
	Extracting  float64
}

func TestPullImage(t *testing.T) {
	sdk, _ := docker.NewDockerClient()
	//尝试拉取镜像
	reader, err := sdk.Client.ImagePull(context.Background(), "phpmyadmin", types.ImagePullOptions{})
	if err != nil {
		fmt.Printf("%v \n", err)
	}

	var progress map[string]*pullImageProgress
	progress = make(map[string]*pullImageProgress)

	out := bufio.NewReader(reader)
	for {
		str, err := out.ReadString('\n')
		if err == io.EOF {
			break
		} else {
			pd := &progressDetail{}
			json.Unmarshal([]byte(str), pd)
			if pd.Status == "Pulling fs layer" {
				progress[pd.Id] = &pullImageProgress{
					Extracting:  0,
					Downloading: 0,
				}
			}
			if pd.ProgressDetail.Total > 0 && pd.Status == "Downloading" {
				progress[pd.Id].Downloading = math.Floor((pd.ProgressDetail.Current / pd.ProgressDetail.Total) * 100)
			}
			if pd.ProgressDetail.Total > 0 && pd.Status == "Extracting" {
				progress[pd.Id].Extracting = math.Floor((pd.ProgressDetail.Current / pd.ProgressDetail.Total) * 100)
			}
			if pd.Status == "Download complete" {
				progress[pd.Id].Downloading = 100
			}
			if pd.Status == "Pull complete" {
				progress[pd.Id].Extracting = 100
			}

			fmt.Printf("%v \n", progress)
		}
	}
}

func TestCreateContainer(t *testing.T) {
	sdk, err := docker.NewDockerClient()
	if err != nil {
		fmt.Printf("%v \n", err)
	}
	builder := sdk.GetContainerCreateBuilder()
	builder.WithImage("portainer/portainer-ce:latest", false)
	builder.WithContainerName("portainer")
	//builder.WithEnv("PMA_HOST", "localmysql")
	builder.WithPort("8000", "8000")
	builder.WithPort("9000", "9000")
	//builder.WithLink("localmysql", "localmysql")
	builder.WithRestart("always")
	builder.WithPrivileged()
	builder.WithVolume("/var/run/docker.sock", "/var/run/docker.sock", "write")
	response, err := builder.Execute()
	if err != nil {
		fmt.Printf("%v \n", err)
	}
	fmt.Printf("%v \n", response.ID)
	err = sdk.Client.ContainerStart(context.Background(), response.ID, types.ContainerStartOptions{})
	if err != nil {
		fmt.Printf("%v \n", err)
	}

}

func TestGetContainer(t *testing.T) {
	sdk, err := docker.NewDockerClient()
	if err != nil {
		fmt.Printf("%v \n", err)
	}
	item, err := sdk.ContainerByField("name", "dpanel-site-50-bDOrc2t6G5", "dpanel-system-48-ULI6AsL1Yw", "dpanel-app-47-xZvGQCce3o")
	//if err != nil {
	//	fmt.Printf("%v \n", err)
	//	return
	//}
	//fmt.Printf("%v \n", item)

	item, err = sdk.ContainerByField("publish", "80", "9000")
	fmt.Printf("%v \n", item)
}

func TestGetContainerLog(t *testing.T) {
	sdk, err := docker.NewDockerClient()
	if err != nil {
		fmt.Printf("%v \n", err)
	}
	filter := filters.NewArgs()
	filter.Add("desired-state", "running")
	filter.Add("desired-state", "shutdown")
	filter.Add("desired-state", "accepted")
	task, err := sdk.Client.TaskList(context.Background(), types.TaskListOptions{
		Filters: filter,
	})
	fmt.Printf("%v \n", task)
	return
	builder := sdk.GetContainerLogBuilder()
	builder.WithContainerId("0bf3c0b9f3d6")
	builder.WithTail(0)
	content, err := builder.Execute()
	fmt.Printf("%v \n", err)
	fmt.Printf("%v \n", content)
}

type progressStream struct {
	Stream string `json:"stream"`
}

type progressImageBuild struct {
	StepTotal   string `json:"stepTotal"`
	StepCurrent string `json:"stepCurrent"`
	Message     string `json:"message"`
}

func TestImageBuild(t *testing.T) {
	sdk, err := docker.NewDockerClient()
	if err != nil {
		fmt.Printf("%v \n", err)
	}
	pg := progressImageBuild{}
	stream := progressStream{}
	str := "{\"stream\":\"Step 2/9 : RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.tuna.tsinghua.edu.cn/g' /etc/apk/repositories\"}"
	json.Unmarshal([]byte(str), &stream)

	field := strings.Fields(stream.Stream)
	if field != nil && field[0] == "Step" {
		step := strings.Split(field[1], "/")
		pg.StepTotal = step[1]
		pg.StepCurrent = step[0]
	}
	pg.Message = stream.Stream
	rs, _ := json.Marshal(pg)
	fmt.Printf("%v \n", string(rs))
	return
	builder := sdk.GetImageBuildBuilder()
	builder.WithZipFilePath("/Users/renchao/Workspace/open-system/artifact-lskypro/data2.zip")
	builder.WithDockerFileContent([]byte("adsfasdfsadf111111"))
	builder.Execute()

}

func TestLoginRegistry(t *testing.T) {
	sdk, err := docker.NewDockerClient()
	if err != nil {
		fmt.Printf("%v \n", err)
	}
	auth, err := sdk.Client.RegistryLogin(context.Background(), registry.AuthConfig{
		Username:      "100009529522",
		Password:      "chaoren945RC",
		ServerAddress: "ccr.ccs.tencentyun.com",
	})
	fmt.Printf("%v \n", err)
	fmt.Printf("%v \n", auth)

	messageChan, errorChan := sdk.Client.Events(context.Background(), types.EventsOptions{})
	for {
		select {
		case messaage := <-messageChan:
			fmt.Printf("%v \n", messaage)
			time.Sleep(time.Second)
		case err := <-errorChan:
			fmt.Printf("%v \n", err.Error())
			time.Sleep(time.Second)
		}
	}
}

func TestImage(t *testing.T) {
	sdk, err := docker.NewDockerClient()
	if err != nil {
		fmt.Printf("%v \n", err)
	}
	result, _, err := sdk.Client.ImageInspectWithRaw(context.Background(), "dddd:latest")
	fmt.Printf("%v \n", result)
	result1, err := sdk.Client.ImageRemove(context.Background(), "phpmyadmin", types.ImageRemoveOptions{})
	fmt.Printf("%v \n", result1)
	fmt.Printf("%v \n", err)
}

func TestChan(t *testing.T) {
	messageQueue := make(chan string, 10)
	ctx := context.WithValue(context.Background(), "message", messageQueue)
	ctx, canel := context.WithCancel(ctx)
	messageQueue <- "abc"

	messageChan := ctx.Value("message").(chan string)

	select {
	case str := <-messageChan:
		fmt.Printf("%v \n", str)
	}
	fmt.Printf("%v \n", canel)
}

type fileItem struct {
	Name     string `json:"name"`
	Typeflag byte   `json:"typeFlag"`
	LinkName string `json:"linkName"`
	Size     int64  `json:"size"`
	Mode     int64  `json:"mode"`
	IsDir    bool   `json:"isDir"`
	ModTime  string `json:"modTime"`
}

func TestCode(t *testing.T) {
	file, _ := os.Open("./fileList.json")
	jsonStr, _ := io.ReadAll(file)

	var fileList []*fileItem
	json.Unmarshal(jsonStr, &fileList)

	path := "/home/site"
	path = strings.TrimSuffix(path, "/") + "/"
	level := strings.Count(path, "/")
	fmt.Printf("%v \n", level)
	for _, item := range fileList {
		if strings.HasPrefix(item.Name, path) {
			pathName := strings.TrimSuffix(item.Name, "/")
			showName := filepath.Base(item.Name)
			if strings.Count(pathName, "/") == level {
				fmt.Printf("%v \n", showName)
			}
		}
	}
}
