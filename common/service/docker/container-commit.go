package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"text/template"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/go-connections/nat"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/google/uuid"
)

const containerCommitMergeLayerThreshold = 100

const containerCommitConfigChangesTemplate = `{{if .Cmd}}CMD {{json .Cmd}}
{{end}}{{if .Entrypoint}}ENTRYPOINT {{json .Entrypoint}}
{{end}}{{range .Env}}ENV {{env .}}
{{end}}{{range portKeys .ExposedPorts}}EXPOSE {{.}}
{{end}}{{with .Healthcheck}}{{if .Test}}HEALTHCHECK{{if gt .Interval 0}} --interval={{.Interval}}{{end}}{{if gt .Timeout 0}} --timeout={{.Timeout}}{{end}}{{if gt .StartPeriod 0}} --start-period={{.StartPeriod}}{{end}}{{if gt .StartInterval 0}} --start-interval={{.StartInterval}}{{end}}{{if gt .Retries 0}} --retries={{.Retries}}{{end}}{{if eq (index .Test 0) "NONE"}} NONE{{else if eq (index .Test 0) "CMD"}} CMD {{json (slice .Test 1)}}{{else if eq (index .Test 0) "CMD-SHELL"}} CMD {{join (slice .Test 1) " "}}{{end}}
{{end}}{{end}}{{range $key := labelKeys .Labels}}LABEL {{quote $key}}={{quote (index $.Labels $key)}}
{{end}}{{range .OnBuild}}ONBUILD {{.}}
{{end}}{{if .StopSignal}}STOPSIGNAL {{.StopSignal}}
{{end}}{{if .User}}USER {{quote .User}}
{{end}}{{if .Volumes}}VOLUME {{json (volumeKeys .Volumes)}}
{{end}}{{if .WorkingDir}}WORKDIR {{quote .WorkingDir}}
{{end}}`

type ContainerCommitOption struct {
	Tag   string
	Merge bool
}

// ContainerCommit preserves DPanel commit metadata, returns the committed image reference,
// and merges the original container directly when requested or before exceeding Docker's layer limit.
func (self Client) ContainerCommit(ctx context.Context, containerID string, option ContainerCommitOption) (string, error) {
	containerInfo, err := self.Client.ContainerInspect(ctx, containerID)
	if err != nil {
		return "", fmt.Errorf("inspect container before commit: %w", err)
	}
	if containerInfo.Config == nil {
		return "", errors.New("container config is empty")
	}
	imageInfo, err := self.Client.ImageInspect(ctx, containerInfo.Image)
	if err != nil {
		return "", fmt.Errorf("inspect container image before commit: %w", err)
	}

	commitName := strings.TrimSpace(option.Tag)
	if commitName == "" && imageInfo.Config != nil {
		commitName = imageInfo.Config.Labels[define.DPanelLabelImageCommitName]
	}
	if commitName == "" {
		imageRepository := strings.SplitN(containerInfo.Config.Image, "@", 2)[0]
		if lastSlash, lastColon := strings.LastIndex(imageRepository, "/"), strings.LastIndex(imageRepository, ":"); lastColon > lastSlash {
			imageRepository = imageRepository[:lastColon]
		}
		containerName := strings.TrimPrefix(strings.TrimSpace(containerInfo.Name), "/")
		if len(containerName) > 121 {
			containerName = containerName[:121]
		}
		commitName = fmt.Sprintf("%s:%s-%s", imageRepository, containerName, uuid.NewString()[24:30])
	}
	metadata := map[string]string{
		define.DPanelLabelImageCommitName: commitName,
	}
	if imageInfo.Config != nil && (imageInfo.Config.Labels[define.DPanelLabelImageCommitName] == "" ||
		imageInfo.Config.Labels[define.DPanelLabelImageCommitName] == commitName) {
		metadata[define.DPanelLabelImageCommitSourceID] = imageInfo.Config.Labels[define.DPanelLabelImageCommitSourceID]
		metadata[define.DPanelLabelImageCommitSourceTag] = imageInfo.Config.Labels[define.DPanelLabelImageCommitSourceTag]
	}
	if metadata[define.DPanelLabelImageCommitSourceID] == "" {
		metadata[define.DPanelLabelImageCommitSourceID] = imageInfo.ID
	}
	if metadata[define.DPanelLabelImageCommitSourceTag] == "" {
		metadata[define.DPanelLabelImageCommitSourceTag] = containerInfo.Config.Image
	}
	shouldFlatten := option.Merge || len(imageInfo.RootFS.Layers) >= containerCommitMergeLayerThreshold
	metadata[define.DPanelLabelImageCommitMerged] = "false"
	if shouldFlatten {
		metadata[define.DPanelLabelImageCommitMerged] = "true"
	}

	changes := make([]string, 0, len(metadata))
	for _, key := range []string{
		define.DPanelLabelImageCommitName,
		define.DPanelLabelImageCommitSourceID,
		define.DPanelLabelImageCommitSourceTag,
		define.DPanelLabelImageCommitMerged,
	} {
		value, err := json.Marshal(metadata[key])
		if err != nil {
			return "", fmt.Errorf("encode container commit metadata %s: %w", key, err)
		}
		changes = append(changes, fmt.Sprintf("LABEL %s=%s", key, string(value)))
	}
	commitOption := container.CommitOptions{
		Reference: commitName,
		Changes:   changes,
	}
	if !shouldFlatten {
		filesystemChanges, err := self.Client.ContainerDiff(ctx, containerInfo.ID)
		if err != nil {
			return "", fmt.Errorf("inspect container filesystem changes: %w", err)
		}
		addedCount := 0
		modifiedCount := 0
		deletedCount := 0
		for _, change := range filesystemChanges {
			switch change.Kind {
			case container.ChangeAdd:
				addedCount++
			case container.ChangeModify:
				modifiedCount++
			case container.ChangeDelete:
				deletedCount++
			}
		}
		commitOption.Comment = "DPanel Commit · No filesystem changes"
		if len(filesystemChanges) > 0 {
			commitOption.Comment = fmt.Sprintf(
				"DPanel Commit · Added %d · Modified %d · Deleted %d",
				addedCount,
				modifiedCount,
				deletedCount,
			)
		}
	}
	resumeAfterCommit := containerInfo.State != nil && containerInfo.State.Running && !containerInfo.State.Paused
	if resumeAfterCommit {
		if err = self.Client.ContainerPause(ctx, containerInfo.ID); err != nil {
			return "", fmt.Errorf("pause container before commit: %w", err)
		}
	}

	if shouldFlatten {
		_, err = self.ContainerCommitFlatten(ctx, containerInfo.ID, commitOption)
	} else {
		_, err = self.Client.ContainerCommit(ctx, containerInfo.ID, commitOption)
	}
	if err != nil {
		if resumeAfterCommit {
			err = errors.Join(err, self.Client.ContainerUnpause(ctx, containerInfo.ID))
		}
		return "", fmt.Errorf("commit container filesystem changes: %w", err)
	}
	if resumeAfterCommit {
		if err = self.Client.ContainerUnpause(ctx, containerInfo.ID); err != nil {
			return "", fmt.Errorf("unpause container after commit: %w", err)
		}
	}
	return commitName, nil
}

// ContainerCommitFlatten creates a new image from the container's complete filesystem
// and configuration without retaining its existing image layers.
func (self Client) ContainerCommitFlatten(ctx context.Context, containerID string, option container.CommitOptions) (container.CommitResponse, error) {
	containerInfo, err := self.Client.ContainerInspect(ctx, containerID)
	if err != nil {
		return container.CommitResponse{}, fmt.Errorf("inspect container before flattened commit: %w", err)
	}
	if containerInfo.Config == nil {
		return container.CommitResponse{}, errors.New("container config is empty")
	}
	imageInfo, err := self.Client.ImageInspect(ctx, containerInfo.Image)
	if err != nil {
		return container.CommitResponse{}, fmt.Errorf("inspect container image before flattened commit: %w", err)
	}
	changeValueReplacer := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		`$`, `\$`,
	)
	quote := func(value string) (string, error) {
		if strings.ContainsAny(value, "\x00\r\n") {
			return "", errors.New("container config value contains an unsupported line break or null character")
		}
		return `"` + changeValueReplacer.Replace(value) + `"`, nil
	}
	configTemplate, err := template.New("container-commit-config").Funcs(template.FuncMap{
		"env": func(value string) (string, error) {
			key, envValue, exists := strings.Cut(value, "=")
			if !exists {
				return "", errors.New("container environment entry has no assignment separator")
			}
			quotedKey, err := quote(key)
			if err != nil {
				return "", err
			}
			quotedValue, err := quote(envValue)
			if err != nil {
				return "", err
			}
			return quotedKey + "=" + quotedValue, nil
		},
		"join": strings.Join,
		"json": func(value any) (string, error) {
			result, err := json.Marshal(value)
			return string(result), err
		},
		"labelKeys":  function.SortedMapKeys[string, string],
		"portKeys":   function.SortedMapKeys[nat.Port, struct{}],
		"quote":      quote,
		"volumeKeys": function.SortedMapKeys[string, struct{}],
	}).Parse(containerCommitConfigChangesTemplate)
	if err != nil {
		return container.CommitResponse{}, fmt.Errorf("parse container commit config template: %w", err)
	}
	var output bytes.Buffer
	if err = configTemplate.Execute(&output, containerInfo.Config); err != nil {
		return container.CommitResponse{}, fmt.Errorf("render container commit config changes: %w", err)
	}
	changes := make([]string, 0, len(option.Changes))
	for change := range strings.SplitSeq(output.String(), "\n") {
		if change = strings.TrimSpace(change); change != "" {
			changes = append(changes, change)
		}
	}
	changes = append(changes, option.Changes...)
	platform := imageInfo.Os + "/" + imageInfo.Architecture
	if imageInfo.Variant != "" {
		platform += "/" + imageInfo.Variant
	}

	exportReader, err := self.Client.ContainerExport(ctx, containerInfo.ID)
	if err != nil {
		return container.CommitResponse{}, fmt.Errorf("export container filesystem: %w", err)
	}
	defer exportReader.Close()

	importReader, err := self.Client.ImageImport(ctx, image.ImportSource{
		Source:     exportReader,
		SourceName: "-",
	}, option.Reference, image.ImportOptions{Message: option.Comment, Changes: changes, Platform: platform})
	if err != nil {
		return container.CommitResponse{}, fmt.Errorf("import container filesystem: %w", err)
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
			return container.CommitResponse{}, fmt.Errorf("read imported container filesystem response: %w", err)
		}
		if message.ErrorDetail != nil && message.ErrorDetail.Message != "" {
			return container.CommitResponse{}, fmt.Errorf("import container filesystem: %s", message.ErrorDetail.Message)
		}
		if message.Error != "" {
			return container.CommitResponse{}, fmt.Errorf("import container filesystem: %s", message.Error)
		}
		if message.Status != "" {
			imageID = message.Status
		}
	}
	if imageID == "" {
		return container.CommitResponse{}, errors.New("imported container image id is empty")
	}
	importedImageInfo, err := self.Client.ImageInspect(ctx, imageID)
	if err != nil {
		return container.CommitResponse{}, fmt.Errorf("inspect imported container image: %w", err)
	}
	return container.CommitResponse{ID: importedImageInfo.ID}, nil
}
