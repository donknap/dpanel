package storage

import (
	"fmt"
	"os"
	"path/filepath"
)

var (
	ROOT_PATH, _ = filepath.Abs("./")
)

type Local struct {
}

func (self Local) Delete(name string) error {
	err := os.Remove(self.GetRealPath(name))
	return err
}

func (self Local) GetSaveRootPath() string {
	return fmt.Sprintf("%s/storage/", ROOT_PATH)
}

func (self Local) GetSavePath(name string) string {
	return fmt.Sprintf("/storage/%s", name)
}

func (self Local) GetRealPath(name string) string {
	return fmt.Sprintf("%s%s", ROOT_PATH, name)
}
