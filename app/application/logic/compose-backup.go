package logic

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/google/uuid"
	"github.com/patrickmn/go-cache"
)

type ComposeBackup struct {
	Id          string
	composeRow  *entity.Compose
	composeData []byte
	runName     string
	fileList    []composeFileBackup
}

type composeFileBackup struct {
	Path     string
	Content  []byte
	FileInfo os.FileInfo
	Link     string
}

func NewComposeBackup(composeRow *entity.Compose) (*ComposeBackup, error) {
	content, err := json.Marshal(composeRow)
	if err != nil {
		return nil, err
	}
	return &ComposeBackup{
		Id:          uuid.NewString(),
		composeRow:  composeRow,
		composeData: content,
		runName:     composeRow.Setting.RunName,
	}, nil
}

func GetComposeBackup(id string, composeRow *entity.Compose) (*ComposeBackup, error) {
	if id == "" {
		return nil, nil
	}
	cacheKey := fmt.Sprintf(storage.CacheKeyComposeBackup, id)
	backup, ok := storage.LoadCache[*ComposeBackup](cacheKey)
	if !ok {
		return nil, nil
	}
	if backup.composeRow.ID != composeRow.ID || backup.composeRow.Name != composeRow.Name || backup.composeRow.Setting.DockerEnvName != composeRow.Setting.DockerEnvName {
		return nil, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted)
	}
	composeFile, overrideFile, err := composeRow.Setting.GetUriPath(composeRow.Name)
	if err != nil {
		return nil, err
	}
	if backup.fileList[1].Path != composeFile || backup.fileList[2].Path != overrideFile {
		return nil, function.ErrorMessage(define.ErrorMessageCommonDataNotFoundOrDeleted)
	}
	storage.Cache.Set(cacheKey, backup, cache.NoExpiration)
	return backup, nil
}

func (self *ComposeBackup) Backup() error {
	composeFile, overrideFile, err := self.composeRow.Setting.GetUriPath(self.composeRow.Name)
	if err != nil {
		return err
	}
	fileList := []string{
		filepath.Join(filepath.Dir(composeFile), define.ComposeDefaultEnvFileName),
		composeFile,
		overrideFile,
	}
	backupList := make([]composeFileBackup, 0, len(fileList))
	for _, filePath := range fileList {
		item := composeFileBackup{Path: filePath}
		item.FileInfo, err = os.Lstat(filePath)
		if errors.Is(err, os.ErrNotExist) {
			backupList = append(backupList, item)
			continue
		}
		if err != nil {
			return err
		}
		if item.FileInfo.Mode()&os.ModeSymlink != 0 {
			item.Link, err = os.Readlink(filePath)
			if err != nil {
				return err
			}
			item.FileInfo, err = os.Stat(filePath)
			if err != nil {
				return err
			}
		}
		item.Content, err = os.ReadFile(filePath)
		if err != nil {
			return err
		}
		backupList = append(backupList, item)
	}
	self.fileList = backupList
	storage.Cache.Set(fmt.Sprintf(storage.CacheKeyComposeBackup, self.Id), self, time.Hour)
	return nil
}

func (self *ComposeBackup) Restore() error {
	cacheKey := fmt.Sprintf(storage.CacheKeyComposeBackup, self.Id)
	composeRow := &entity.Compose{}
	if err := json.Unmarshal(self.composeData, composeRow); err != nil {
		return fmt.Errorf("restore compose backup %q: %w", self.Id, err)
	}
	var err error
	for _, item := range self.fileList {
		if item.FileInfo == nil {
			if removeErr := os.Remove(item.Path); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				err = errors.Join(err, removeErr)
			}
			continue
		}
		if item.Link != "" {
			if removeErr := os.Remove(item.Path); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				err = errors.Join(err, removeErr)
				continue
			}
			if linkErr := os.Symlink(item.Link, item.Path); linkErr != nil {
				err = errors.Join(err, linkErr)
				continue
			}
		}
		if writeErr := os.WriteFile(item.Path, item.Content, item.FileInfo.Mode().Perm()); writeErr != nil {
			err = errors.Join(err, writeErr)
			continue
		}
		err = errors.Join(err, os.Chmod(item.Path, item.FileInfo.Mode()))
		err = errors.Join(err, os.Chtimes(item.Path, item.FileInfo.ModTime(), item.FileInfo.ModTime()))
	}
	if err != nil {
		return fmt.Errorf("restore compose backup %q: %w", self.Id, err)
	}
	if self.composeRow.ID > 0 {
		composeRow.ID = self.composeRow.ID
		composeRow.Setting.RunName = self.runName
		composeRow.Setting.Dangling = self.fileList[1].FileInfo == nil || self.fileList[1].FileInfo.Size() == 0
		if err = dao.Compose.Save(composeRow); err != nil {
			return fmt.Errorf("restore compose backup %q: %w", self.Id, err)
		}
	}
	storage.Cache.Delete(cacheKey)
	return nil
}
