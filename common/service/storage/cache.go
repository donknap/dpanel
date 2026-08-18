package storage

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/patrickmn/go-cache"
)

var (
	CacheKeyExplorerUsername              = "explorer:%s:uid:%d"
	CacheKeyExplorerAfs                   = "explorer:%s"
	CacheKeyCommonUserInfo                = "user:%d"
	CacheKeyCommonServerStartTime         = "server:startTime"
	CacheKeyXkStorageInfo                 = "xk:storageInfo"
	CacheKeyLoginFailed                   = "login:failed:%s"
	CacheKeyOauthState                    = "oauth:state:%s"
	CacheKeyOauthCode                     = "oauth:code:%s"
	CacheKeySetting                       = "setting:%s"
	CacheKeySettingLocale                 = fmt.Sprintf(CacheKeySetting, "locale")
	CacheKeyContainerUpgradeCheck         = "container:upgrade:check:%s:%s"  // 镜像更新检查结果：Docker 环境、容器 ID
	CacheKeyContainerUpgradeLogs          = "container:upgrade:logs:%d"      // 计划任务执行日志：计划 ID
	CacheKeyContainerUpgradeContainerLock = "container:upgrade:lock:%s:%s"   // 容器操作锁：Docker 环境、容器名
	CacheKeyContainerUpgradeTaskLock      = "container:upgrade:task:lock:%d" // 计划任务执行锁：任务 ID
	CacheKeyImageRootFs                   = "image:rootfs:%s"
	CacheKeyDockerStatus                  = "docker:status:%s"
	CacheKeyDockerEvents                  = "docker:events"
	CacheKeyDockerContainerRuntime        = "docker:container:runtime:%s:%s"
	CacheKeyConsoleData                   = "console:data:%s" // 用于脚本存储一些自定义数据
	CacheKeyDockerEventJob                = "docker:event:%s:%s"
	CacheKeyRsaKey                        = "rsa:key"
	CacheKeyRsaPub                        = "rsa:pub"
	CacheKeyAttach                        = "attach:%s"
	CacheKeyAsset                         = "asset"
)

var (
	Cache = cache.New(cache.DefaultExpiration, 5*time.Minute)
)

func LoadCache[T any](key string) (value T, ok bool) {
	item, exists := Cache.Get(key)
	if !exists {
		return value, false
	}
	value, ok = item.(T)
	if !ok {
		Cache.Delete(key)
	}
	return value, ok
}

// LoadCacheOrStore returns the cached value or calls saver on a cache miss.
// Concurrent cache misses may execute saver more than once.
func LoadCacheOrStore[T any](key string, saver func() T) (value T, loaded bool) {
	if value, loaded = LoadCache[T](key); loaded {
		return value, true
	}
	return saver(), false
}

type Mutex struct {
	key    string
	locked atomic.Bool
}

func NewMutex(key string) *Mutex {
	return &Mutex{key: key}
}

func (self *Mutex) TryLock() bool {
	if self == nil || self.key == "" || !self.locked.CompareAndSwap(false, true) {
		return false
	}
	if err := Cache.Add(self.key, true, cache.NoExpiration); err != nil {
		self.locked.Store(false)
		return false
	}
	return true
}

func (self *Mutex) Unlock() {
	if self == nil || !self.locked.CompareAndSwap(true, false) {
		return
	}
	Cache.Delete(self.key)
}
