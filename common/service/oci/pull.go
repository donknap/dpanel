package oci

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
)

// PullImage writes the selected image filesystem as a tar archive and returns its labels.
func PullImage(ctx context.Context, reference, archivePath string, platform v1.Platform, maxBytes int64) (map[string]string, error) {
	if platform.OS == "" || platform.Architecture == "" || maxBytes <= 0 {
		return nil, fmt.Errorf("OCI platform and archive size limit are required")
	}
	image, err := crane.Pull(reference, crane.WithContext(ctx), crane.WithPlatform(&platform))
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
	written, err := io.Copy(file, io.LimitReader(stream, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("export OCI image filesystem: %w", err)
	}
	if written > maxBytes {
		return nil, fmt.Errorf("OCI image filesystem archive exceeds %d bytes", maxBytes)
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
