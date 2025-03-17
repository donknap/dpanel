package storage

import (
	"github.com/patrickmn/go-cache"
	"time"
)

var (
	Cache = cache.New(cache.DefaultExpiration, 5*time.Minute)
)
