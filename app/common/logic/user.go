package logic

import (
	"errors"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/golang-jwt/jwt/v5"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
	"gorm.io/datatypes"
	"gorm.io/gen"
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
	return []byte(docker.BuilderAuthor + facade.GetConfig().GetString("app.name"))
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

func (self User) GetUserByUsername(username string) (*entity.Setting, error) {
	return dao.Setting.Where(dao.Setting.GroupName.Eq(SettingGroupUser)).
		Where(gen.Cond(datatypes.JSONQuery("value").Equals(username, "username"))...).First()
}

func (self User) CreateUser(username string, userRole string) (*entity.Setting, error) {
	user, err := self.GetUserByUsername(username)
	if err == nil && user != nil {
		return nil, errors.New("用户已存在")
	}

	user = &entity.Setting{
		GroupName: SettingGroupUser,
		Name:      userRole,
		Value: &accessor.SettingValueOption{
			Password:   "",
			Username:   username,
			Email:      "",
			UserStatus: SettingGroupUserStatusEnable,
		},
	}

	err = dao.Setting.Create(user)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (self User) GetResetUserInfoToken(user *entity.Setting) (string, error) {
	jwtSecret := self.GetJwtSecret()
	ttlSeconds := facade.GetConfig().GetInt("jwt.reset_user_info_ttl_seconds")
	if ttlSeconds <= 0 {
		ttlSeconds = 3600
	}
	jwtClaims := jwt.NewWithClaims(jwt.SigningMethodHS256, UserInfo{
		UserId:       0,
		Username:     user.Value.Username,
		RoleIdentity: user.Name,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Second * time.Duration(ttlSeconds))),
		},
	})

	return jwtClaims.SignedString(jwtSecret)
}

func (self User) ValidateResetUserInfoToken(token string) (*UserInfo, error) {
	myUserInfo := &UserInfo{}
	jwtToken, err := jwt.ParseWithClaims(token, myUserInfo, func(t *jwt.Token) (interface{}, error) {
		return self.GetJwtSecret(), nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		return nil, err
	}
	if jwtToken.Valid {
		return myUserInfo, nil
	}
	return nil, errors.New("token验证失败")
}
