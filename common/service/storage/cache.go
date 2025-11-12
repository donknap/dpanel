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
)

var (
	Cache = cache.New(cache.DefaultExpiration, 5*time.Minute)
)
