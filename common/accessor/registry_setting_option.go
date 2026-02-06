package accessor

import (
	"log/slog"

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
	var err error
	if self.Username != "" && self.Password != "" {
		password, err = function.RSADecode(self.Password, []byte(facade.GetConfig().GetString("app.name")))
		if err != nil {
			slog.Debug("registry setting decode password", "error", err)
		}
	}
	return self.Username, password, self.Username != "" && password != ""
}
