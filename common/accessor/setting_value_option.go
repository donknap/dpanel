package accessor

import (
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/donknap/dpanel/common/service/docker"
	"time"
)

type SettingValueOption struct {
	Username                 string                     `json:"username,omitempty"`
	Password                 string                     `json:"password,omitempty"`
	Email                    string                     `json:"email,omitempty"`
	UserStatus               uint8                      `json:"userStatus,omitempty"`
	UserRemark               string                     `json:"userRemark,omitempty"`
	RegisterAt               *time.Time                 `json:"registerAt,omitempty"`
	Docker                   map[string]*docker.Client  `json:"docker,omitempty"`
	DiskUsage                *DiskUsage                 `json:"diskUsage,omitempty"`
	TwoFa                    *TwoFa                     `json:"twoFa,omitempty"`
	IgnoreCheckUpgrade       IgnoreCheckUpgrade         `json:"ignoreCheckUpgrade,omitempty"`
	ContainerCheckAllUpgrade *ContainerCheckAllUpgrade  `json:"containerCheckAllUpgrade,omitempty"`
	DPanelInfo               *container.InspectResponse `json:"DPanelInfo,omitempty"`
	ThemeConfig              *ThemeConfig               `json:"theme,omitempty"`
	ThemeUserConfig          *ThemeUserConfig           `json:"themeUser,omitempty"`
	DnsApi                   []DnsApi                   `json:"dnsApi,omitempty"`
	EmailServer              *EmailServer               `json:"emailServer,omitempty"`
	Tag                      []Tag                      `json:"tag,omitempty"`
}

type IgnoreCheckUpgrade []string

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

type ThemeConfig struct {
	MainMenu        string `json:"mainMenu,omitempty"`
	Algorithm       string `json:"algorithm,omitempty"`
	Compact         string `json:"compact,omitempty"`
	FontSizeConsole int    `json:"fontSizeConsole,omitempty"`
	FontSize        int    `json:"fontSize,omitempty"`
	SideMenu        string `json:"sideMenu,omitempty"`
	TablePageSize   string `json:"tablePageSize,omitempty"`
}

type ThemeUserConfig struct {
	Token   map[string]interface{} `json:"token"`
	BgImage struct {
		Src    string `json:"src,omitempty"`
		Width  string `json:"width,omitempty"`
		Height string `json:"height,omitempty"`
		Top    int    `json:"top,omitempty"`
		Left   int    `json:"left,omitempty"`
		Right  int    `json:"right,omitempty"`
		Bottom int    `json:"bottom,omitempty"`
	} `json:"bgImage,omitempty"`
	SiteLink      []ThemeUserConfigSiteLink `json:"siteLink,omitempty"`
	SiteCopyright string                    `json:"siteCopyright,omitempty"`
	SiteTitle     string                    `json:"siteTitle"`
	SiteLogo      string                    `json:"siteLogo,omitempty"`
	LoginLogo     string                    `json:"loginLogo,omitempty"`
}

type ThemeUserConfigSiteLink struct {
	Href  string `json:"href,omitempty"`
	Title string `json:"title,omitempty"`
}

type DnsApi struct {
	Title      string           `json:"title,omitempty"`
	ServerName string           `json:"serverName,omitempty"`
	Env        []docker.EnvItem `json:"env,omitempty"`
}

type EmailServer struct {
	Host   string   `json:"host,omitempty" binding:"required"`
	Port   int      `json:"port,omitempty" binding:"required"`
	Email  string   `json:"email,omitempty" binding:"required"`
	Code   string   `json:"code,omitempty" binding:"required"`
	Status []string `json:"status,omitempty"`
}

type ContainerCheckAllUpgrade struct {
	Upgrade   int                 `json:"upgrade"`
	Failed    int                 `json:"failed"`
	Local     int                 `json:"local"`
	Ignore    int                 `json:"ignore"`
	Container []map[string]string `json:"container"`
}

type TagItem struct {
	Name        string `json:"name,omitempty"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	Url         string `json:"url,omitempty"`
	Icon        string `json:"icon,omitempty"`
}

type Tag struct {
	EnableShowGroup bool      `json:"enableShowGroup"`
	Tag             string    `json:"tag,omitempty" binding:"required"` // 为分组显示时，tag 为分组标题
	TagColor        string    `json:"tagColor,omitempty"`
	Item            []TagItem `json:"item,omitempty"`
}
