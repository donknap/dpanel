package accessor

import (
	"github.com/donknap/dpanel/common/function"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
)

type RegistrySettingOption struct {
	Username   string   `json:"username"`
	Password   string   `json:"password"`
	Email      string   `json:"email"`
	Proxy      []string `json:"proxy"`
	EnableHttp bool     `json:"enableHttp"`
}

func (self RegistrySettingOption) Auth() (username, password string, ok bool) {
	if self.Username != "" && self.Password != "" {
		password, _ = function.AseDecode(facade.GetConfig().GetString("app.name"), self.Password)
	}
	return self.Username, password, self.Username != "" && password != ""
}
