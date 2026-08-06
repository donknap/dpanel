package container

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/types/define"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

type containerRollback struct {
	platform      *v1.Platform
	containerInfo container.InspectResponse
	id            string
	name          string
	originalName  string
	enableBak     bool
	prepared      bool
}

func (self *containerRollback) commit(dockerSdk *docker.Client) error {
	if self.containerInfo.ID == "" {
		return nil
	}
	if self.enableBak {
		if self.id == "" {
			return self.build(dockerSdk, self.name)
		}
		if _, err := dockerSdk.Client.ContainerInspect(dockerSdk.Ctx, self.id); err != nil {
			if !errdefs.IsNotFound(err) {
				return err
			}
			self.id = ""
			return self.build(dockerSdk, self.name)
		}
		return nil
	}
	if self.id != "" {
		if err := dockerSdk.Client.ContainerRemove(dockerSdk.Ctx, self.id, container.RemoveOptions{Force: true}); err != nil && !errdefs.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func (self *containerRollback) prepare(dockerSdk *docker.Client, containerName string) error {
	self.originalName = strings.TrimPrefix(strings.TrimSpace(containerName), "/")
	if self.containerInfo.ID == "" {
		containerInfo, err := dockerSdk.Client.ContainerInspect(dockerSdk.Ctx, containerName)
		if errdefs.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		self.containerInfo = containerInfo
	}
	containerInfo, err := dockerSdk.ContainerInspectCompat(self.containerInfo)
	if err != nil {
		return err
	}
	self.containerInfo = containerInfo
	if originalContainerName := strings.TrimPrefix(strings.TrimSpace(self.containerInfo.Name), "/"); originalContainerName != "" {
		self.originalName = originalContainerName
	}
	if self.name == "" {
		self.name = fmt.Sprintf("%s-bak-%s", containerName, time.Now().Format(define.DateYmdHis))
	}
	currentContainerInfo, err := dockerSdk.Client.ContainerInspect(dockerSdk.Ctx, self.containerInfo.ID)
	if errdefs.IsNotFound(err) {
		self.id = ""
		self.prepared = true
		return nil
	}
	if err != nil {
		return err
	}
	originalContainerName := strings.TrimPrefix(strings.TrimSpace(self.containerInfo.Name), "/")
	currentContainerName := strings.TrimPrefix(strings.TrimSpace(currentContainerInfo.Name), "/")
	if currentContainerName != originalContainerName {
		return fmt.Errorf("container name changed from %s to %s", originalContainerName, currentContainerName)
	}

	self.prepared = true
	err = dockerSdk.Client.ContainerStop(dockerSdk.Ctx, self.containerInfo.ID, container.StopOptions{})
	if err != nil && !errdefs.IsNotModified(err) && !errdefs.IsNotFound(err) {
		return err
	}
	if currentContainerInfo, err = dockerSdk.Client.ContainerInspect(dockerSdk.Ctx, self.containerInfo.ID); err != nil {
		if errdefs.IsNotFound(err) {
			self.id = ""
			return nil
		}
		return err
	}
	currentContainerName = strings.TrimPrefix(strings.TrimSpace(currentContainerInfo.Name), "/")
	if currentContainerName != originalContainerName {
		return fmt.Errorf("container name changed from %s to %s", originalContainerName, currentContainerName)
	}

	err = dockerSdk.Client.ContainerRename(dockerSdk.Ctx, self.containerInfo.ID, self.name)
	if err == nil {
		self.id = self.containerInfo.ID
		return nil
	}
	if errdefs.IsNotFound(err) {
		self.id = ""
		return nil
	}
	if !errdefs.IsConflict(err) {
		return err
	}

	containerID := self.containerInfo.ID
	if len(containerID) > 12 {
		containerID = containerID[:12]
	}
	self.name = self.name + "-" + containerID
	if renameErr := dockerSdk.Client.ContainerRename(dockerSdk.Ctx, self.containerInfo.ID, self.name); renameErr != nil {
		if errdefs.IsNotFound(renameErr) {
			self.id = ""
			return nil
		}
		return errors.Join(err, renameErr)
	}
	self.id = self.containerInfo.ID
	return nil
}

func (self *containerRollback) build(dockerSdk *docker.Client, containerName string) error {
	if self.containerInfo.Config == nil {
		return errors.New("backup container config is empty")
	}
	networkingConfig := &network.NetworkingConfig{EndpointsConfig: map[string]*network.EndpointSettings{}}
	if self.containerInfo.NetworkSettings != nil {
		networkingConfig.EndpointsConfig = self.containerInfo.NetworkSettings.Networks
	}
	containerConfig := *self.containerInfo.Config
	containerConfig.Image = self.containerInfo.Image
	var hostConfig *container.HostConfig
	if self.containerInfo.HostConfig != nil {
		value := *self.containerInfo.HostConfig
		hostConfig = &value
	}
	response, err := dockerSdk.Client.ContainerCreate(
		dockerSdk.Ctx,
		&containerConfig,
		hostConfig,
		networkingConfig,
		self.platform,
		containerName,
	)
	if err != nil {
		return err
	}
	self.id = response.ID
	return nil
}

func (self *containerRollback) restore(dockerSdk *docker.Client, newContainerID string) (string, error) {
	var resultErr error
	if newContainerID != "" {
		if err := dockerSdk.Client.ContainerRemove(dockerSdk.Ctx, newContainerID, container.RemoveOptions{Force: true}); err != nil && !errdefs.IsNotFound(err) {
			resultErr = errors.Join(resultErr, err)
		}
	}
	if self.containerInfo.ID != "" && self.prepared {
		restoreContainerID := self.id
		if restoreContainerID == "" {
			restoreContainerID = self.containerInfo.ID
		}
		buildContainer := false
		containerInfo, err := dockerSdk.Client.ContainerInspect(dockerSdk.Ctx, restoreContainerID)
		if err == nil {
			if strings.TrimPrefix(strings.TrimSpace(containerInfo.Name), "/") != self.originalName {
				if err = dockerSdk.Client.ContainerRename(dockerSdk.Ctx, containerInfo.ID, self.originalName); err != nil {
					resultErr = errors.Join(resultErr, err)
					restoreContainerID = ""
					if errdefs.IsNotFound(err) {
						buildContainer = true
					}
				}
			}
		} else if errdefs.IsNotFound(err) {
			restoreContainerID = ""
			buildContainer = true
		} else {
			resultErr = errors.Join(resultErr, err)
			restoreContainerID = ""
		}
		if buildContainer {
			if err = self.build(dockerSdk, self.originalName); err != nil {
				resultErr = errors.Join(resultErr, err)
			} else {
				restoreContainerID = self.id
			}
		}
		if restoreContainerID != "" && self.containerInfo.State != nil && self.containerInfo.State.Running {
			if err = dockerSdk.Client.ContainerStart(dockerSdk.Ctx, restoreContainerID, container.StartOptions{}); err != nil && !errdefs.IsNotModified(err) {
				resultErr = errors.Join(resultErr, err)
			}
		}
	}
	if self.originalName == "" {
		return "", errors.Join(resultErr, errors.New("original container name is empty"))
	}
	containerInfo, err := dockerSdk.Client.ContainerInspect(dockerSdk.Ctx, self.originalName)
	if err != nil {
		return "", errors.Join(resultErr, fmt.Errorf("inspect final container %s: %w", self.originalName, err))
	}
	return containerInfo.ID, resultErr
}
