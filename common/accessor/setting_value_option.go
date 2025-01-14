package accessor

import (
	"github.com/docker/docker/api/types"
	"github.com/donknap/dpanel/common/service/docker"
	"time"
)

type SettingValueOption struct {
	Username           string                    `json:"username,omitempty"`
	Password           string                    `json:"password,omitempty"`
	ServerIp           string                    `json:"serverIp,omitempty"`
	Docker             map[string]*docker.Client `json:"docker,omitempty"`
	DiskUsage          *DiskUsage                `json:"diskUsage,omitempty"`
	TwoFa              *TwoFa                    `json:"twoFa,omitempty"`
	IgnoreCheckUpgrade []string                  `json:"ignoreCheckUpgrade,omitempty"`
	DPanelInfo         *types.ContainerJSON      `json:"DPanelInfo,omitempty"`
	Theme              *Theme                    `json:"theme,omitempty"`
	ThemeUser          *ThemeUser                `json:"themeUser,omitempty"`
}

type DiskUsage struct {
	Usage     *types.DiskUsage `json:"usage,omitempty"`
	UpdatedAt time.Time        `json:"updatedAt,omitempty"`
}

type TwoFa struct {
	Secret string `json:"secret,omitempty"`
	Enable bool   `json:"enable,omitempty"`
	Email  string `json:"email,omitempty"`
	QrCode string `json:"qrCode,omitempty"`
}

type Theme struct {
	MainMenu        string `json:"mainMenu,omitempty"`
	Algorithm       string `json:"algorithm,omitempty"`
	Compact         string `json:"compact,omitempty"`
	FontSizeConsole int    `json:"fontSizeConsole,omitempty"`
	FontSize        int    `json:"fontSize,omitempty"`
	SideMenu        string `json:"sideMenu,omitempty"`
	TablePageSize   string `json:"tablePageSize,omitempty"`
}

type ThemeUser struct {
	Token   map[string]interface{} `json:"token"`
	BgImage struct {
		Src    string `json:"src,omitempty"`
		Width  string `json:"width,omitempty"`
		Height string `json:"height,omitempty"`
		Top    string `json:"top,omitempty"`
		Left   string `json:"left,omitempty"`
		Right  string `json:"right,omitempty"`
		Bottom string `json:"bottom,omitempty"`
	} `json:"bgImage"`
	SiteLink []struct {
		Href  string `json:"href,omitempty"`
		Title string `json:"title,omitempty"`
	} `json:"siteLink"`
	SiteCopyright string `json:"siteCopyright,omitempty"`
	SiteTitle     string `json:"siteTitle,omitempty"`
	SiteLogo      string `json:"siteLogo,omitempty"`
}
