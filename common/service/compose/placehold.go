package compose

import (
	"strings"
	"time"

	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/define"
)

// 仅在应用商店中的配置文件 data.yml 中支持
const (
	PlaceholderAppName         = "%APP_NAME%"
	PlaceholderAppVersion      = "%APP_VERSION%"
	PlaceholderAppTaskName     = "%APP_TASK_NAME%"
	PlaceholderCurrentUsername = "%CURRENT_USERNAME%"
	PlaceholderCurrentDate     = "%CURRENT_DATE%"
	PlaceholderXkStoragePath   = "%XK_STORAGE_INFO%"
)

var ValueReplaceTable = []function.Replacer[string]{
	func(v *string) {
		*v = function.StringReplaceAll(*v, PlaceholderCurrentDate, time.Now().Format(define.DateYmdHis))
	},
}

var EnvItemReplaceTable = []function.Replacer[docker.EnvItem]{
	func(item *docker.EnvItem) {
		if !strings.Contains(item.Value, PlaceholderXkStoragePath) {
			return
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
		return
	},
}
