package tests

import (
	"context"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/donknap/dpanel/common/service/docker"
	"testing"
)

func TestContainerRemove(t *testing.T) {
	sdk, _ := docker.NewDockerClient()
	err := sdk.Client.ContainerStop(context.Background(), "phpmyadmin", container.StopOptions{})
	err = sdk.Client.ContainerRemove(context.Background(), "phpmyadmin", types.ContainerRemoveOptions{})
	fmt.Printf("%v \n", err)

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
