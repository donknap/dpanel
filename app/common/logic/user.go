package logic

import (
	"errors"
	"fmt"
	"time"

	"github.com/docker/go-units"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/patrickmn/go-cache"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
	"gorm.io/datatypes"
	"gorm.io/gen"
	"gorm.io/gorm"
)

var (
	loginSetting = accessor.Login{
		FailedEnable:   true,
		FailedTotal:    5,
		FailedLockTime: 900,
	}
)

type UserInfo struct {
	Fd               string                          `json:"fd"`
	UserId           int32                           `json:"userId"`
	Username         string                          `json:"username"`
	SecurityPassword bool                            `json:"securityPassword"`
	Email            string                          `json:"email"`
	RoleIdentity     string                          `json:"roleIdentity"`
	Permission       *accessor.PermissionValueOption `json:"permission"`
	AutoLogin        bool                            `json:"autoLogin"`
	jwt.RegisteredClaims
}

type User struct {
}

func (self User) GetBuiltInPublicUsername() string {
	return facade.GetConfig().GetString("system.permission.default_username")
}

func (self User) GetMd5Password(password, username, salt string) string {
	return function.Md5(password + username + salt)
}

func (self User) GetTokenId(user *entity.Setting) string {
	return function.Sha256([]byte(fmt.Sprintf("%d:%s", user.ID, user.Value.Salt)))
}

func (self User) CheckLock(username string) error {
	Setting{}.GetByKey(SettingGroupSetting, SettingGroupSettingLogin, &loginSetting)
	if !loginSetting.FailedEnable {
		return nil
	}
	cacheKey := fmt.Sprintf(storage.CacheKeyLoginFailed, username)
	if item, ok := storage.Cache.Get(cacheKey); ok {
		if v, ok := item.(int); ok && v >= loginSetting.FailedTotal {
			if loginSetting.FailedLockTime == -1 {
				return function.ErrorMessage(define.ErrorMessageUserFailedLockForever)
			}
			return function.ErrorMessage(define.ErrorMessageUserFailedLock, "time", units.HumanDuration(time.Duration(loginSetting.FailedLockTime)*time.Second))
		}
	}
	return nil
}

func (self User) Lock(username string, failed bool) {
	Setting{}.GetByKey(SettingGroupSetting, SettingGroupSettingLogin, &loginSetting)
	if !loginSetting.FailedEnable {
		return
	}
	var lockTime time.Duration
	if loginSetting.FailedLockTime == -1 {
		lockTime = cache.DefaultExpiration
	} else {
		lockTime = time.Duration(loginSetting.FailedLockTime) * time.Second
	}
	cacheKey := fmt.Sprintf(storage.CacheKeyLoginFailed, username)
	if failed {
		var err error
		if _, err = storage.Cache.IncrementInt(cacheKey, 1); err != nil {
			storage.Cache.Set(cacheKey, 1, lockTime)
		}
	} else {
		storage.Cache.Delete(cacheKey)
	}
}

func (self User) GetUserByUsername(username string) (*entity.Setting, error) {
	return dao.Setting.Where(dao.Setting.GroupName.Eq(SettingGroupUser)).
		Where(gen.Cond(datatypes.JSONQuery("value").Equals(username, "username"))...).First()
}

func (self User) GetFounderUser() (*entity.Setting, error) {
	founder, err := dao.Setting.Where(dao.Setting.GroupName.Eq(SettingGroupUser)).
		Where(dao.Setting.Name.Eq(SettingGroupUserFounder)).
		First()
	if err != nil {
		return nil, err
	}
	if founder.Value == nil || founder.Value.UserStatus == SettingGroupUserStatusDisable {
		return nil, errors.New("dpanel founder user is disabled")
	}
	return founder, nil
}

func (self User) CreateFounderUser(username string, password string) (*entity.Setting, error) {
	if founder, err := dao.Setting.Where(dao.Setting.GroupName.Eq(SettingGroupUser)).
		Where(dao.Setting.Name.Eq(SettingGroupUserFounder)).
		First(); err == nil && founder != nil {
		return nil, function.ErrorMessage(define.ErrorMessageUserFounderExists)
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	registerAt := time.Now()
	founder := &entity.Setting{
		GroupName: SettingGroupUser,
		Name:      SettingGroupUserFounder,
		Value: &accessor.SettingValueOption{
			UserSetting: accessor.UserSetting{
				Username:   username,
				UserStatus: SettingGroupUserStatusEnable,
				RegisterAt: &registerAt,
			},
		},
	}
	if password != "" {
		founder.Value.Salt = uuid.NewString()
		founder.Value.Password = self.GetMd5Password(password, username, founder.Value.Salt)
	}
	if err := dao.Setting.Create(founder); err != nil {
		return nil, err
	}
	return founder, nil
}

func (self User) GetUserOauthToken(user *entity.Setting, autoLogin bool) (string, error) {
	now := time.Now()
	userInfo := UserInfo{
		UserId:       user.ID,
		Username:     user.Value.Username,
		RoleIdentity: user.Name,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:       self.GetTokenId(user),
			IssuedAt: jwt.NewNumericDate(now),
		},
		AutoLogin: autoLogin,
	}
	var rsaKeyContent []byte
	if v, ok := storage.Cache.Get(storage.CacheKeyRsaKey); ok {
		rsaKeyContent = v.([]byte)
	}
	privateKey, err := function.RSAParsePrivateKey(rsaKeyContent)
	if err != nil {
		return "", err
	}
	jwtClaims := jwt.NewWithClaims(jwt.SigningMethodRS512, userInfo)
	// 登录成功后在缓存中写入用户数据，用于后端主动退出用户时使用
	storage.Cache.Set(fmt.Sprintf(storage.CacheKeyCommonUserInfo, userInfo.UserId), userInfo, cache.DefaultExpiration)
	return jwtClaims.SignedString(privateKey)
}
