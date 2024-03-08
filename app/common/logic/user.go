package logic

import (
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/golang-jwt/jwt/v5"
	"github.com/we7coreteam/w7-rangine-go-support/src/facade"
)

type UserToken struct {
	UserId       int    `json:"userId"`
	RoleIdentity string `json:"roleIdentity"`
	jwt.RegisteredClaims
}

type User struct {
}

func (self User) GetJwtSecret() []byte {
	return []byte(docker.BuilderAuthor + facade.GetConfig().GetString("app.name"))
}
