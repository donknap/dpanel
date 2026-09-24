package logic

import (
	"archive/tar"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/errdefs"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/types/define"
)

type BuildxTarget struct {
	Exists  bool
	Proxy   string
	Name    string
	ID      string
	Running bool
}

func (self ImageBuildx) GetTarget(sdk *docker.Client) (BuildxTarget, error) {
	builderName := fmt.Sprintf(define.DockerBuilderName, sdk.Name)
	containerName := "buildx_buildkit_" + builderName + "0"
	result := BuildxTarget{Name: containerName}
	info, err := sdk.Client.ContainerInspect(sdk.Ctx, containerName)
	if errdefs.IsNotFound(err) {
		return result, nil
	}
	if err != nil {
		return result, err
	}
	if info.Name != "/"+containerName {
		return result, fmt.Errorf("unexpected buildkit container name %q", info.Name)
	}
	result.Exists = true
	result.ID = info.ID
	result.Running = info.State != nil && info.State.Running
	if info.Config != nil {
		for _, item := range info.Config.Env {
			if value, ok := strings.CutPrefix(item, "HTTP_PROXY="); ok {
				result.Proxy = value
				break
			}
			if value, ok := strings.CutPrefix(item, "HTTPS_PROXY="); ok && result.Proxy == "" {
				result.Proxy = value
			}
		}
	}
	return result, nil
}

func (self ImageBuildx) ReadTargetConfig(sdk *docker.Client, target BuildxTarget) (string, error) {
	archive, _, err := sdk.Client.CopyFromContainer(sdk.Ctx, target.ID, "/etc/buildkit/buildkitd.toml")
	if err != nil {
		return "", fmt.Errorf("read buildkit config from %s: %w", target.Name, err)
	}
	defer archive.Close()
	reader := tar.NewReader(archive)
	header, err := reader.Next()
	if err != nil {
		return "", fmt.Errorf("read buildkit config archive: %w", err)
	}
	if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA {
		return "", fmt.Errorf("buildkit config is not a regular file")
	}
	content, err := io.ReadAll(io.LimitReader(reader, 4*1024*1024+1))
	if err != nil {
		return "", err
	}
	if len(content) > 4*1024*1024 {
		return "", fmt.Errorf("buildkit config exceeds 4 MiB")
	}
	return string(content), nil
}

func (self ImageBuildx) RemoveTarget(sdk *docker.Client, force bool) error {
	target, err := self.GetTarget(sdk)
	if err != nil {
		return err
	}
	if target.Exists {
		if err := sdk.Client.ContainerRemove(sdk.Ctx, target.ID, container.RemoveOptions{Force: force}); err != nil && !errdefs.IsNotFound(err) {
			return err
		}
	}
	volumeName := target.Name + "_state"
	if err := sdk.Client.VolumeRemove(sdk.Ctx, volumeName, false); err != nil && !errdefs.IsNotFound(err) {
		return err
	}
	return nil
}

func (self ImageBuildx) PruneTarget(sdk *docker.Client) error {
	target, err := self.GetTarget(sdk)
	if err != nil {
		return err
	}
	if !target.Exists {
		return fmt.Errorf("buildkit container %s does not exist", target.Name)
	}
	if !target.Running {
		if err := sdk.Client.ContainerStart(sdk.Ctx, target.ID, container.StartOptions{}); err != nil {
			return err
		}
	}
	var pruneErr error
	for attempt := 0; attempt < 5; attempt++ {
		_, pruneErr = sdk.ContainerExecResult(sdk.Ctx, target.ID, container.ExecOptions{Cmd: []string{"buildctl", "prune", "--all"}})
		if pruneErr == nil {
			break
		}
		if attempt < 4 && !target.Running {
			time.Sleep(time.Second)
		} else {
			break
		}
	}
	if !target.Running {
		if err := sdk.Client.ContainerStop(sdk.Ctx, target.ID, container.StopOptions{}); err != nil {
			return fmt.Errorf("stop buildkit after pruning: %w (prune: %v)", err, pruneErr)
		}
	}
	return pruneErr
}
