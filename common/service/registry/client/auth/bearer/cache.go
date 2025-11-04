package bearer

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

func newCache(capacity int) *cache {
	return &cache{
		latency:  10,
		capacity: capacity,
		cache:    map[string]*token{},
	}
}

type cache struct {
	sync.RWMutex
	latency  int // second, the network latency in case that when the token is checked it doesn't expire but it does when used
	capacity int // the capacity of the cache map
	cache    map[string]*token
}

func (c *cache) get(scopes []*scope) *token {
	c.RLock()
	defer c.RUnlock()
	token := c.cache[c.key(scopes)]
	if token == nil {
		return nil
	}
	expired, _ := c.expired(token)
	if expired {
		token = nil
	}
	return token
}

func (c *cache) set(scopes []*scope, token *token) {
	c.Lock()
	defer c.Unlock()
	// exceed the capacity, empty some elements: all expired token will be removed,
	// if no expired token, move the earliest one
	if len(c.cache) >= c.capacity {
		var candidates []string
		var earliestKey string
		var earliestExpireTime time.Time
		for key, value := range c.cache {
			expired, expireAt := c.expired(value)
			// expired
			if expired {
				candidates = append(candidates, key)
				continue
			}
			// doesn't expired
			if len(earliestKey) == 0 || expireAt.Before(earliestExpireTime) {
				earliestKey = key
				earliestExpireTime = expireAt
				continue
			}
		}
		if len(candidates) == 0 {
			candidates = append(candidates, earliestKey)
		}
		for _, candidate := range candidates {
			delete(c.cache, candidate)
		}
	}
	c.cache[c.key(scopes)] = token
}

func (c *cache) key(scopes []*scope) string {
	var strs []string
	for _, scope := range scopes {
		strs = append(strs, scope.String())
	}
	return strings.Join(strs, "#")
}

// return whether the token is expired or not and the expired time
func (c *cache) expired(token *token) (bool, time.Time) {
	// check time whether empty
	if len(token.IssuedAt) == 0 {
		slog.Warn("token issued time is empty, return expired to refresh token")
		return true, time.Time{}
	}

	issueAt, err := time.Parse(time.RFC3339, token.IssuedAt)
	if err != nil {
		slog.Error(fmt.Sprintf("failed to parse the issued at time of token %s: %v", token.IssuedAt, err))
		return true, time.Time{}
	}
	expireAt := issueAt.Add(time.Duration(token.ExpiresIn-c.latency) * time.Second)
	return expireAt.Before(time.Now()), expireAt
}
