package logic

import (
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/golang-jwt/jwt/v5"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
	"time"
)

var (
	maxFailedCount = 5
	lockTime       = 15 * time.Minute
)

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
	if item, ok := storage.Cache.Get(username); ok {
		if v, ok := item.(int); ok && v >= maxFailedCount {
			return true
		}
	}
	return false
}

func (self User) Lock(username string, failed bool) {
	if failed {
		var err error
		if _, err = storage.Cache.IncrementInt(username, 1); err != nil {
			storage.Cache.Set(username, 1, lockTime)
		}
	} else {
		storage.Cache.Delete(username)
	}
}
