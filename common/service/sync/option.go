package sync

import (
	"time"

	"github.com/donknap/dpanel/common/service/docker"
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

type GitOption struct {
	RemoveGit bool
	MaxBytes  int64
}

type ZipOption struct {
	Root      string
	StripRoot bool
}

type OciOption struct {
	Platform       v1.Platform
	ExpectedLabels map[string]string
	DockerClient   *docker.Client
}

type UrlOption struct {
	// Timeout 和 MaxBytes 为零时分别使用 30 分钟和 20 GiB，负值无效。
	Timeout             time.Duration
	MaxBytes            int64
	RequireExistingFile bool
}
