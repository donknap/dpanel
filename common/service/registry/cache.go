package registry

import (
	"net/http"
	"sync"
	"time"
)

var cache sync.Map

type cacheItem struct {
	header     http.Header
	body       []byte
	expireTime time.Time
}
