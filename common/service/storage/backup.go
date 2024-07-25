package storage

import (
	"fmt"
)

func (self Local) GetBackupPath(name string) string {
	return fmt.Sprintf("%s/backup/%s", self.GetStorageLocalPath(), name)
}
