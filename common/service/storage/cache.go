package storage

import (
	"time"

	"github.com/patrickmn/go-cache"
)

var (
	CacheKeyImageRemoteList       = "image:remoteTag:%s"
	CacheKeyExplorerUsername      = "explorer:%s:uid:%d"
	CacheKeyCommonUserInfo        = "user:%d"
	CacheKeyCommonServerStartTime = "server:startTime"
	CacheKeyXkStorageInfo         = "xk:storageInfo"
	CacheKeyLoginFailed           = "login:failed:%s"
	CacheKeySetting               = "setting:%s"
)

var (
	Cache = cache.New(cache.DefaultExpiration, 5*time.Minute)
)
