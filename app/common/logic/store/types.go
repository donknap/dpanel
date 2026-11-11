package store

import (
	"context"
	"errors"

	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/service/docker"
)

var ErrUnsupported = errors.New("store operation is not supported")

type SourceOption struct {
	DockerClient *docker.Client
}

type StoreSource interface {
	Check(ctx context.Context) (string, error)
	Sync(ctx context.Context, sourceURL string) ([]accessor.StoreAppDetail, error)
	GetList() ([]accessor.StoreAppDetail, error)
	PreInstall(detail accessor.StoreAppInstallDetail) error
}
