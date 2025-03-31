package compose

import (
	"errors"
	"github.com/donknap/dpanel/common/function"
	"strings"
	"time"
)

// 仅在应用商店中的配置文件 data.yml 中支持
const (
	ContainerDefaultName = "%CONTAINER_DEFAULT_NAME%"
	CurrentUsername      = "%CURRENT_USERNAME%"
	TaskIndex            = "%TASK_INDEX%"
	CurrentDate          = "%CURRENT_DATE%"
)

type ReplaceFunc func(placeholder string) (string, error)
type ReplaceTable map[string]ReplaceFunc

func NewReplaceTable(rt ...ReplaceTable) ReplaceTable {
	defaultTable := ReplaceTable{
		ContainerDefaultName: func(placeholder string) (string, error) {
			return "", nil
		},
		CurrentUsername: func(placeholder string) (string, error) {
			return "", errors.New("not implemented")
		},
		CurrentDate: func(placeholder string) (string, error) {
			return time.Now().Format(function.YmdHis), nil
		},
	}
	for _, item := range rt {
		for k, v := range item {
			defaultTable[k] = v
		}
	}

	return defaultTable
}

func (self ReplaceTable) Replace(replace *string) error {
	var err error
	for key, replaceFunc := range self {
		if v, err := replaceFunc(key); err == nil {
			*replace = strings.Replace(*replace, key, v, -1)
		}
	}
	return err
}
