package logic

import (
	"errors"
	"fmt"
	"time"

	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/google/uuid"
)

type AttachDownloadTask struct {
	FilePath            string
	DeleteAfterDownload bool
}

type Attach struct{}

func (self Attach) PreDownload(filePath string, expireTime time.Duration, deleteAfterDownload ...bool) (string, error) {
	if filePath == "" {
		return "", errors.New("filepath cannot be empty")
	}
	token := uuid.New().String()
	cacheKey := fmt.Sprintf(storage.CacheKeyAttach, token)
	shouldDeleteAfterDownload := len(deleteAfterDownload) > 0 && deleteAfterDownload[0]
	storage.Cache.Set(cacheKey, &AttachDownloadTask{
		FilePath:            filePath,
		DeleteAfterDownload: shouldDeleteAfterDownload,
	}, expireTime)

	return fmt.Sprintf("%s?id=%s", function.RouterApiUri("/common/attach/download"), token), nil
}
