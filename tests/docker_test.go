package tests

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/donknap/dpanel/common/service/docker"
	"io"
	"math"
	"strings"
	"testing"
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

			fmt.Printf("%v \n", progress["249ff3a7bbe6"])
		}
	}
}

func TestCreateContainer(t *testing.T) {
	sdk, err := docker.NewDockerClient()
	if err != nil {
		fmt.Printf("%v \n", err)
	}
	builder := sdk.GetContainerCreateBuilder()
	builder.WithImage("phpmyadmin:latest")
	builder.WithContainerName("phpmyadmin")
	builder.WithEnv("PMA_HOST", "localmysql")
	builder.WithPort("8011", "80")
	builder.WithLink("localmysql", "localmysql")
	builder.WithAlwaysRestart()
	builder.WithPrivileged()
	builder.WithVolume("/Users/renchao/Downloads", "/home/abc")
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
	item, err := sdk.ContainerByName("phpmyadmin1")
	if err != nil {
		fmt.Printf("%v \n", err)
		return
	}
	fmt.Printf("%v \n", item)
}

func TestCode(t *testing.T) {
	image := "phpmyadmin:"
	fmt.Printf("%v \n", strings.Split(image, ":"))
	a := strings.Split(image, ":")
	fmt.Printf("%v \n", a[1])
}
