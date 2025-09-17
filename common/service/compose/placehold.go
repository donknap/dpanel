package compose

import (
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/storage"
	"strings"
	"time"
)

// 仅在应用商店中的配置文件 data.yml 中支持
const (
	ContainerDefaultName = "%CONTAINER_DEFAULT_NAME%"
	WebsiteDefaultPath   = "%WEBSITE_DEFAULT_PATH%"
	CurrentUsername      = "%CURRENT_USERNAME%"
	CurrentDate          = "%CURRENT_DATE%"
	XkStoragePath        = "%XK_STORAGE_INFO%"
)

type ReplaceFunc func(item *docker.EnvItem) error
type ReplaceTable []ReplaceFunc

func NewReplaceTable(rt ...ReplaceFunc) ReplaceTable {
	defaultTable := ReplaceTable{
		func(item *docker.EnvItem) error {
			if !strings.Contains(item.Value, ContainerDefaultName) {
				return nil
			}
			item.Value = strings.ReplaceAll(item.Value, ContainerDefaultName, function.GetRandomString(12))
			return nil
		},
		func(item *docker.EnvItem) error {
			if !strings.Contains(item.Value, CurrentDate) {
				return nil
			}
			item.Value = strings.ReplaceAll(item.Value, CurrentDate, time.Now().Format(function.YmdHis))
			return nil
		},
		func(item *docker.EnvItem) error {
			if !strings.Contains(item.Value, XkStoragePath) {
				return nil
			}
			item.Value = ""
			if v, ok := storage.Cache.Get(storage.CacheKeyXkStorageInfo); ok {
				item.Rule.Option = function.PluckArrayWalk(v.([]string), func(item string) (docker.ValueItem, bool) {
					return docker.ValueItem{
						Name:  item,
						Value: item,
					}, true
				})
			}
			return nil
		},
		func(item *docker.EnvItem) error {
			if !strings.Contains(item.Value, WebsiteDefaultPath) {
				return nil
			}
			item.Value = strings.ReplaceAll(item.Value, WebsiteDefaultPath, storage.Local{}.GetDefaultWebsitePath())
			return nil
		},
	}
	for _, item := range rt {
		defaultTable = append(defaultTable, item)
	}

	return defaultTable
}

func (self ReplaceTable) Replace(item *docker.EnvItem) error {
	var err error
	for _, replaceFunc := range self {
		err = replaceFunc(item)
		if err != nil {
			return err
		}
	}
	return err
}
