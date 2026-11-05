package store

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/donknap/dpanel/common/accessor"
	sourcesync "github.com/donknap/dpanel/common/service/sync"
)

type Portainer struct{ source }

func (self Portainer) Sync(ctx context.Context, sourceURL string) ([]accessor.StoreAppDetail, error) {
	if err := os.MkdirAll(self.path, 0755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(self.tempPath, 0700); err != nil {
		return nil, err
	}
	file, err := os.CreateTemp(self.tempPath, ".sync-*")
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := os.Remove(file.Name()); err != nil && !os.IsNotExist(err) {
			slog.Warn("remove store sync temporary file", "error", err)
		}
	}()
	if err = file.Close(); err != nil {
		return nil, err
	}
	if err = sourcesync.ByUrl(ctx, sourceURL, file.Name(), sourcesync.UrlOption{}); err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	storeSourceMutex.Lock()
	defer storeSourceMutex.Unlock()
	if err = os.Rename(file.Name(), filepath.Join(self.path, "template.json")); err != nil {
		return nil, err
	}
	// 当前仅下载模板文件，尚未解析为应用索引。
	appList := make([]accessor.StoreAppDetail, 0)
	if err = self.saveList(appList); err != nil {
		return nil, err
	}
	return appList, nil
}

func (self Portainer) GetList() ([]accessor.StoreAppDetail, error) {
	return nil, ErrUnsupported
}

func (self Portainer) PreInstall(detail accessor.StoreAppInstallDetail) error {
	return ErrUnsupported
}
