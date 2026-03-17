package docker

import (
	"context"
	"io"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/swarm"
	"github.com/donknap/dpanel/common/function"
)

func (self Client) ServiceLogs(ctx context.Context, serviceId string, options container.LogsOptions) (io.ReadCloser, error) {
	serviceInfo, _, err := self.Client.ServiceInspectWithRaw(ctx, serviceId, swarm.ServiceInspectOptions{})
	if err != nil {
		return nil, err
	}
	rawStream, err := self.Client.ServiceLogs(ctx, serviceId, options)
	if err != nil {
		return nil, err
	}
	if serviceInfo.Spec.TaskTemplate.ContainerSpec != nil && serviceInfo.Spec.TaskTemplate.ContainerSpec.TTY {
		return rawStream, nil
	} else {
		return function.DockerCombinedStream(rawStream), nil
	}
}

func (self Client) TaskLogs(ctx context.Context, taskId string, options container.LogsOptions) (io.ReadCloser, error) {
	taskInfo, _, err := self.Client.TaskInspectWithRaw(ctx, taskId)
	if err != nil {
		return nil, err
	}
	rawStream, err := self.Client.TaskLogs(ctx, taskId, options)
	if err != nil {
		return nil, err
	}
	if taskInfo.Spec.ContainerSpec != nil && taskInfo.Spec.ContainerSpec.TTY {
		return rawStream, nil
	} else {
		return function.DockerCombinedStream(rawStream), nil
	}
}
