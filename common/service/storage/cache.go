package storage

import (
	"fmt"
	"time"

	"github.com/patrickmn/go-cache"
)

var (
	CacheKeyExplorerUsername      = "explorer:%s:uid:%d"
	CacheKeyCommonUserInfo        = "user:%d"
	CacheKeyCommonServerStartTime = "server:startTime"
	CacheKeyXkStorageInfo         = "xk:storageInfo"
	CacheKeyLoginFailed           = "login:failed:%s"
	CacheKeySetting               = "setting:%s"
	CacheKeySettingLocale         = fmt.Sprintf(CacheKeySetting, "locale")
	CacheKeyImageDigest           = "image:digest:%s"
	CacheKeyImageRootFs           = "image:rootfs:%s"
	CacheKeyDockerStatus          = "docker:status:%s"
	CacheKeyConsoleData           = "console:data:%s" // 用于脚本存储一些自定义数据
	CacheKeyCronTaskStatus        = "cron:task:status:%d"
	// CacheKeyDockerEventJob        = "docker:event:%s:%s"
)

var (
	Cache = cache.New(cache.DefaultExpiration, 5*time.Minute)
)
