package docker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"

	"github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
)

type ContainerCommitOption struct {
	Merge  bool
	Tag    string
	Labels map[string]string
}

// ContainerCommit returns the image used by the container when its filesystem is unchanged.
// Otherwise, it commits the current filesystem and optionally merges it into a single layer.
func (self Client) ContainerCommit(ctx context.Context, containerID string, option ContainerCommitOption) (string, error) {
	containerInfo, err := self.Client.ContainerInspect(ctx, containerID)
	if err != nil {
		return "", err
	}
	changes, err := self.Client.ContainerDiff(ctx, containerID)
	if err != nil {
		return "", fmt.Errorf("inspect container filesystem changes: %w", err)
	}
	if len(changes) == 0 && !option.Merge && len(option.Labels) == 0 {
		if option.Tag != "" {
			if err = self.Client.ImageTag(ctx, containerInfo.Image, option.Tag); err != nil {
				return "", fmt.Errorf("tag unchanged container image: %w", err)
			}
		}
		return containerInfo.Image, nil
	}

	commitOption := container.CommitOptions{
		Changes: make([]string, 0, len(option.Labels)),
	}
	for key, value := range option.Labels {
		commitOption.Changes = append(commitOption.Changes, fmt.Sprintf("LABEL %s=%s", key, value))
	}
	if !option.Merge {
		commitOption.Reference = option.Tag
	}
	commitResponse, err := self.Client.ContainerCommit(ctx, containerID, commitOption)
	if err != nil {
		return "", fmt.Errorf("commit container filesystem changes: %w", err)
	}
	if !option.Merge {
		return commitResponse.ID, nil
	}

	mergedImageID, err := self.containerCommitMerge(ctx, containerID, option.Tag, commitOption.Changes)
	if err != nil {
		_, cleanupErr := self.Client.ImageRemove(ctx, commitResponse.ID, image.RemoveOptions{})
		if cleanupErr != nil && !errdefs.IsNotFound(cleanupErr) {
			err = errors.Join(err, cleanupErr)
		}
		return "", err
	}
	if _, removeErr := self.Client.ImageRemove(ctx, commitResponse.ID, image.RemoveOptions{}); removeErr != nil && !errdefs.IsNotFound(removeErr) {
		slog.Warn("remove merged container commit image", "imageId", commitResponse.ID, "error", removeErr)
	}
	return mergedImageID, nil
}

func (self Client) containerCommitMerge(ctx context.Context, containerID, tag string, changes []string) (string, error) {
	exportReader, err := self.Client.ContainerExport(ctx, containerID)
	if err != nil {
		return "", fmt.Errorf("export container filesystem: %w", err)
	}
	defer exportReader.Close()

	importReader, err := self.Client.ImageImport(ctx, image.ImportSource{
		Source:     exportReader,
		SourceName: "-",
	}, tag, image.ImportOptions{Changes: changes})
	if err != nil {
		return "", fmt.Errorf("import container filesystem: %w", err)
	}
	defer importReader.Close()

	imageID := ""
	decoder := json.NewDecoder(importReader)
	for {
		message := struct {
			Status      string `json:"status"`
			Error       string `json:"error"`
			ErrorDetail *struct {
				Message string `json:"message"`
			} `json:"errorDetail"`
		}{}
		if err = decoder.Decode(&message); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return "", fmt.Errorf("read imported container filesystem response: %w", err)
		}
		if message.ErrorDetail != nil && message.ErrorDetail.Message != "" {
			return "", fmt.Errorf("import container filesystem: %s", message.ErrorDetail.Message)
		}
		if message.Error != "" {
			return "", fmt.Errorf("import container filesystem: %s", message.Error)
		}
		if message.Status != "" {
			imageID = message.Status
		}
	}
	if imageID == "" {
		return "", errors.New("imported container image id is empty")
	}
	imageInfo, err := self.Client.ImageInspect(ctx, imageID)
	if err != nil {
		return "", fmt.Errorf("inspect imported container image: %w", err)
	}
	return imageInfo.ID, nil
}
