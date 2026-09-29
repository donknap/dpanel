package context

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/errdefs"
	containerexec "github.com/donknap/dpanel/common/service/exec/container"
)

type target struct {
	Exists  bool
	Proxy   string
	Name    string
	ID      string
	Running bool
}

func (self *contextService) getTarget() (target, error) {
	containerName := "buildx_buildkit_" + self.builderName + "0"
	result := target{Name: containerName}
	info, err := self.sdk.Client.ContainerInspect(self.sdk.Ctx, containerName)
	if errors.Is(err, os.ErrNotExist) || errdefs.IsNotFound(err) {
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

func (self *contextService) readTargetConfig(target target) (string, error) {
	archive, _, err := self.sdk.Client.CopyFromContainer(self.sdk.Ctx, target.ID, "/etc/buildkit/buildkitd.toml")
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

func (self *contextService) removeTarget(force bool) error {
	target, err := self.getTarget()
	if err != nil {
		return err
	}
	if err := self.removeContainer(target, force); err != nil {
		return err
	}
	volumeName := target.Name + "_state"
	if err := self.sdk.Client.VolumeRemove(self.sdk.Ctx, volumeName, false); err != nil && !errdefs.IsNotFound(err) {
		return err
	}
	return nil
}

func (self *contextService) removeContainer(target target, force bool) error {
	if !target.Exists {
		return nil
	}
	if err := self.sdk.Client.ContainerRemove(self.sdk.Ctx, target.ID, container.RemoveOptions{Force: force}); err != nil && !errdefs.IsNotFound(err) {
		return err
	}
	return nil
}

func (self *contextService) pruneTarget() error {
	target, err := self.getTarget()
	if err != nil {
		return err
	}
	if !target.Exists {
		return nil
	}
	if !target.Running {
		if err := self.sdk.Client.ContainerStart(self.sdk.Ctx, target.ID, container.StartOptions{}); err != nil {
			return err
		}
	}
	var pruneErr error
	for attempt := 0; attempt < 5; attempt++ {
		_, pruneErr = containerexec.QuickRun(self.sdk.Ctx, self.sdk.Client, target.ID, "buildctl", "prune", "--all")
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
		if err := self.sdk.Client.ContainerStop(self.sdk.Ctx, target.ID, container.StopOptions{}); err != nil {
			return fmt.Errorf("stop buildkit after pruning: %w (prune: %v)", err, pruneErr)
		}
	}
	return pruneErr
}
