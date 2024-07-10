package storage

import (
	"fmt"
	"github.com/we7coreteam/w7-rangine-go-support/src/facade"
)

func (self Local) GetBackupPath(name string) string {
	return fmt.Sprintf("%s/backup/%s", facade.GetConfig().Get("storage.local.path"), name)
}
