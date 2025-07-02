package storage

import (
	"github.com/patrickmn/go-cache"
	"time"
)

var (
	CacheKeyImageRemoteList  = "image:remoteTag:%s"
	CacheKeyExplorerUsername = "explorer:%s:uid:%d"
	CacheKeyCommonUserInfo   = "user:%d"
	CacheKeyXkStorageInfo    = "xk:storageInfo"
	CacheKeyLoginFailed      = "login:failed:%s"
)

var (
	Cache = cache.New(cache.DefaultExpiration, 5*time.Minute)
)
