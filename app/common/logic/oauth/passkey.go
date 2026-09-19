package oauth

import (
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	commonLogic "github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/service/storage"
)

var passkeyExchangeLock sync.Mutex

type Passkey struct {
}

type PasskeyCode struct {
	UserID    int32
	ExpiresAt time.Time
}

func (self Passkey) Item() (Item, bool) {
	return Item{}, false
}

func (self Passkey) Authorize(request *http.Request) (string, error) {
	return "", errors.New("passkey authorize is not supported")
}

func (self Passkey) Exchange(option ExchangeOption) (string, error) {
	setting := accessor.Passkey{}
	if !(commonLogic.Setting{}).GetByKey(commonLogic.SettingGroupSetting, commonLogic.SettingGroupSettingPasskey, &setting) || !setting.Enable {
		return "", errors.New("passkey is not enabled")
	}

	passkeyExchangeLock.Lock()
	defer passkeyExchangeLock.Unlock()
	cacheKey := fmt.Sprintf(storage.CacheKeyOauthCode, option.Code)
	item, exists := storage.Cache.Get(cacheKey)
	if !exists {
		return "", errors.New("passkey oauth code is invalid")
	}
	code, ok := item.(*PasskeyCode)
	storage.Cache.Delete(cacheKey)
	if !ok || time.Now().After(code.ExpiresAt) {
		return "", errors.New("passkey oauth code is invalid")
	}
	user, err := (commonLogic.Setting{}).GetValueById(code.UserID)
	if err != nil || user.Value == nil || user.GroupName != commonLogic.SettingGroupUser || user.Value.UserStatus == commonLogic.SettingGroupUserStatusDisable {
		return "", errors.New("passkey user is disabled")
	}
	if user.Name != commonLogic.SettingGroupUserFounder {
		return "", errors.New("passkey user is not founder")
	}
	return (commonLogic.User{}).GetUserOauthToken(user, option.AutoLogin)
}
