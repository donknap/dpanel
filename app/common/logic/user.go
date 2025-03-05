package logic

import (
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/golang-jwt/jwt/v5"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
	"sync"
	"time"
)

var (
	UserFailedMap  = sync.Map{}
	maxFailedCount = 5
	lockTime       = 10 * time.Minute
)

type UserFailedItem struct {
	LastLogin time.Time `json:"lastLogin"`
	Failed    int       `json:"failed"`
}

func (self UserFailedItem) Max() bool {
	return self.Failed >= maxFailedCount && time.Now().Before(self.LastLogin.Add(lockTime))
}

type UserInfo struct {
	Fd           string `json:"fd"`
	UserId       int32  `json:"userId"`
	Username     string `json:"username"`
	RoleIdentity string `json:"roleIdentity"`
	jwt.RegisteredClaims
}

type User struct {
}

func (self User) GetJwtSecret() []byte {
	return []byte(docker.BuilderAuthor + facade.GetConfig().GetString("app.name") + facade.GetConfig().GetString("jwt.secret"))
}

func (self User) GetMd5Password(password string, key string) string {
	return function.GetMd5(password + key)
}

func (self User) CheckLock(username string) bool {
	if item, ok := UserFailedMap.Load(username); ok {
		if v, ok := item.(UserFailedItem); ok && v.Max() {
			return true
		}
	}
	return false
}

func (self User) Lock(username string, failed bool) {
	if failed {
		reset := true
		if item, ok := UserFailedMap.LoadAndDelete(username); ok {
			if v, ok := item.(UserFailedItem); ok && v.Failed < maxFailedCount {
				reset = false
				v.Failed++
				v.LastLogin = time.Now()
				UserFailedMap.Store(username, v)
			}
		}
		if reset {
			UserFailedMap.Store(username, UserFailedItem{
				LastLogin: time.Now(),
				Failed:    1,
			})
		}
	} else {
		UserFailedMap.Delete(username)
	}
}
