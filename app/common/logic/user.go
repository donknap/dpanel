package logic

import (
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/golang-jwt/jwt/v5"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
	"gorm.io/datatypes"
	"gorm.io/gen"
	"time"
)

var (
	maxFailedCount = 5
	lockTime       = 15 * time.Minute
)

type UserInfo struct {
	Fd               string                          `json:"fd"`
	UserId           int32                           `json:"userId"`
	Username         string                          `json:"username"`
	SecurityPassword bool                            `json:"securityPassword"`
	Email            string                          `json:"email"`
	RoleIdentity     string                          `json:"roleIdentity"`
	Permission       *accessor.PermissionValueOption `json:"permission"`
	jwt.RegisteredClaims
}

type User struct {
}

func (self User) GetJwtSecret() []byte {
	return []byte(docker.BuilderAuthor + facade.GetConfig().GetString("app.name") + facade.GetConfig().GetString("jwt.secret"))
}

func (self User) GetBuiltInPublicUsername() string {
	return facade.GetConfig().GetString("common.public_user_name")
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

func (self User) GetUserByUsername(username string) (*entity.Setting, error) {
	return dao.Setting.Where(dao.Setting.GroupName.Eq(SettingGroupUser)).
		Where(gen.Cond(datatypes.JSONQuery("value").Equals(username, "username"))...).First()
}

func (self User) GetUserOauthToken(user *entity.Setting, autoLogin bool) (string, error) {
	var expireAddTime time.Duration
	if autoLogin {
		expireAddTime = time.Hour * 24 * 30
	} else {
		expireAddTime = time.Hour * 24
	}

	jwtSecret := self.GetJwtSecret()
	jwtClaims := jwt.NewWithClaims(jwt.SigningMethodHS256, UserInfo{
		UserId:       user.ID,
		Username:     user.Value.Username,
		RoleIdentity: user.Name,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expireAddTime)),
		},
	})
	return jwtClaims.SignedString(jwtSecret)
}
