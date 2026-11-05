package oci

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/donknap/dpanel/common/service/docker"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
)

type PullOption struct {
	Platform     v1.Platform
	DockerClient *docker.Client
}

// PullImage writes the selected image filesystem as a tar archive and returns its labels.
func PullImage(ctx context.Context, reference, archivePath string, option PullOption) (map[string]string, error) {
	platform := option.Platform
	if platform.OS == "" && platform.Architecture == "" {
		platform = v1.Platform{OS: "linux", Architecture: "amd64"}
	}
	if platform.OS == "" || platform.Architecture == "" {
		return nil, fmt.Errorf("OCI platform is required")
	}
	var image v1.Image
	var err error
	if option.DockerClient == nil {
		image, err = crane.Pull(reference, crane.WithContext(ctx), crane.WithPlatform(&platform))
	} else {
		_, err = option.DockerClient.ImagePull(ctx, reference, docker.ImagePullOption{Platform: platform.String()})
		if err != nil {
			return nil, err
		}
		var imageReference name.Reference
		imageReference, err = name.ParseReference(reference)
		if err != nil {
			return nil, err
		}
		image, err = daemon.Image(imageReference, daemon.WithClient(option.DockerClient.Client), daemon.WithContext(ctx), daemon.WithUnbufferedOpener())
	}
	if err != nil {
		return nil, fmt.Errorf("pull OCI image %s: %w", reference, err)
	}
	config, err := image.ConfigFile()
	if err != nil {
		return nil, fmt.Errorf("read OCI image config: %w", err)
	}
	if config.OS != platform.OS || config.Architecture != platform.Architecture {
		return nil, fmt.Errorf("OCI image platform is %s/%s, expected %s/%s", config.OS, config.Architecture, platform.OS, platform.Architecture)
	}

	parent := filepath.Dir(archivePath)
	if err = os.MkdirAll(parent, 0o700); err != nil {
		return nil, err
	}
	file, err := os.CreateTemp(parent, ".oci-*")
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = file.Close()
		_ = os.Remove(file.Name())
	}()
	stream := mutate.Extract(image)
	defer stream.Close()
	_, err = io.Copy(file, stream)
	if err != nil {
		return nil, fmt.Errorf("export OCI image filesystem: %w", err)
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	if err = file.Close(); err != nil {
		return nil, err
	}
	if err = os.Rename(file.Name(), archivePath); err != nil {
		return nil, err
	}
	return config.Config.Labels, nil
}
