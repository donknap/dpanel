package cache

import (
	"encoding/json"
	"github.com/donknap/dpanel/common/function"
	"sync"
	"time"
)

type CacheEntity struct {
	Payload    string
	ExpireTime time.Time
}

type ReqCache struct {
	cacheMap        sync.Map
	defaultCacheTtl time.Duration
	secret          string
}

func NewReqCache(secret string, defaultCacheTtl time.Duration) *ReqCache {
	return &ReqCache{
		defaultCacheTtl: defaultCacheTtl,
		secret:          secret,
	}
}

func (rc *ReqCache) SetDefaultCacheTtl(ttl time.Duration) {
	rc.defaultCacheTtl = ttl
}

func (rc *ReqCache) Set(key string, target interface{}) error {
	str, err := json.Marshal(target)
	if err != nil {
		return err
	}
	encryptResult, err := function.AseEncode(rc.secret, string(str))
	if err != nil {
		return err
	}

	rc.cacheMap.Store(key, CacheEntity{
		Payload:    encryptResult,
		ExpireTime: time.Now().Add(rc.defaultCacheTtl),
	})
	return nil
}

func (rc *ReqCache) Get(key string, target interface{}) error {
	val, exists := rc.cacheMap.Load(key)
	if exists {
		entity := val.(CacheEntity)
		if time.Now().After(entity.ExpireTime) {
			rc.Del(key)
			return nil
		}

		decryptResult, err := function.AseDecode(rc.secret, entity.Payload)
		if err != nil {
			return err
		}
		err = json.Unmarshal([]byte(decryptResult), &target)
		if err != nil {
			return err
		}
		return nil
	}
	return nil
}

func (rc *ReqCache) Del(key string) {
	rc.cacheMap.Delete(key)
}
