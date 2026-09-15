package logic

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/datatypes"
	"gorm.io/gen"
)

const passwordHashPrefix = "bcrypt-sha256:"

func passwordInput(password string) []byte {
	sum := sha256.Sum256([]byte(password))
	return []byte(hex.EncodeToString(sum[:]))
}

func (self User) HashPassword(password string) (string, error) {
	// 先生成固定长度摘要，兼容超过 bcrypt 72 字节限制的已有密码。
	hash, err := bcrypt.GenerateFromPassword(passwordInput(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return passwordHashPrefix + string(hash), nil
}

func (self User) NeedsPasswordUpgrade(hash string) bool {
	return !strings.HasPrefix(hash, passwordHashPrefix)
}

func (self User) UpgradePassword(user *entity.Setting, password string) error {
	hash, err := self.HashPassword(password)
	if err != nil {
		return err
	}
	value := *user.Value
	value.Password = hash
	// 迁移时再次核对旧凭据，避免覆盖并发的改密、重置或停用操作。
	result, err := dao.Setting.Where(dao.Setting.ID.Eq(user.ID),
		dao.Setting.GroupName.Eq(user.GroupName), dao.Setting.Name.Eq(user.Name)).
		Where(gen.Cond(datatypes.JSONQuery("value").Equals(user.Value.Password, "password"))...).
		Where(gen.Cond(datatypes.JSONQuery("value").Equals(user.Value.Username, "username"))...).
		Where(gen.Cond(datatypes.JSONQuery("value").Equals(user.Value.UserStatus, "userStatus"))...).
		Updates(&entity.Setting{Value: &value})
	if err != nil {
		return err
	}
	if result.RowsAffected != 1 {
		return errors.New("user credentials changed during login")
	}
	user.Value = &value
	return nil
}

func (self User) CheckPassword(password, username, hash string) bool {
	if hash == "" {
		return false
	}
	if hash, ok := strings.CutPrefix(hash, passwordHashPrefix); ok {
		return bcrypt.CompareHashAndPassword([]byte(hash), passwordInput(password)) == nil
	}
	if strings.HasPrefix(hash, "$2") {
		return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
	}
	legacy := self.GetMd5Password(password, username)
	return len(hash) == 32 && subtle.ConstantTimeCompare([]byte(hash), []byte(legacy)) == 1
}
