package logic

import (
	"github.com/donknap/dpanel/app/common/logic"
	"github.com/golang-jwt/jwt/v5"
	"time"
)

type User struct {
}

func (self User) GetAuth(expireTime time.Time) (string, error) {
	currentUser, err := logic.Setting{}.GetValue(logic.SettingGroupUser, logic.SettingGroupUserFounder)
	if err != nil {
		return "", err
	}
	jwtSecret := logic.User{}.GetJwtSecret()
	jwtClaims := jwt.NewWithClaims(jwt.SigningMethodHS256, logic.UserInfo{
		UserId:       currentUser.ID,
		Username:     currentUser.Value.Username,
		RoleIdentity: currentUser.Name,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expireTime),
		},
	})
	return jwtClaims.SignedString(jwtSecret)
}
