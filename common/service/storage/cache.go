package storage

import (
	"github.com/patrickmn/go-cache"
	"time"
)

var (
	CacheKeyImageRemoteList = "image:remoteTag:%s"
)

var (
	Cache = cache.New(cache.DefaultExpiration, 5*time.Minute)
)
