package accessor

import (
	"time"

	"github.com/docker/docker/api/types"
	types2 "github.com/donknap/dpanel/common/service/docker/types"
	types3 "github.com/donknap/dpanel/common/types"
)

type SettingValueOption struct {
	Username                    string                       `json:"username,omitempty"`
	Password                    string                       `json:"password,omitempty"`
	Email                       string                       `json:"email,omitempty"`
	UserStatus                  uint8                        `json:"userStatus,omitempty"`
	UserRemark                  string                       `json:"userRemark,omitempty"`
	RegisterAt                  *time.Time                   `json:"registerAt,omitempty"`
	Docker                      map[string]*types2.DockerEnv `json:"docker,omitempty"`
	DiskUsage                   *DiskUsage                   `json:"diskUsage,omitempty"`
	TwoFa                       *TwoFa                       `json:"twoFa,omitempty"`
	ContainerCheckIgnoreUpgrade ContainerCheckIgnoreUpgrade  `json:"containerCheckIgnoreUpgrade,omitempty"`
	ContainerCheckAllUpgrade    *ContainerCheckAllUpgrade    `json:"containerCheckAllUpgrade,omitempty"`
	DPanelInfo                  *types3.DPanelInfo           `json:"DPanelInfo,omitempty"`
	ThemeConfig                 *ThemeConfig                 `json:"themeConfig,omitempty"`
	ThemeUserConfig             *ThemeUserConfig             `json:"themeUserConfig,omitempty"`
	ThemeConsoleConfig          *ThemeConsoleConfig          `json:"themeConsoleConfig,omitempty"`
	DnsApi                      []DnsApi                     `json:"dnsApi,omitempty"`
	Notification                *Notification                `json:"notification,omitempty"`
	Tag                         []Tag                        `json:"tag,omitempty"`
	Login                       *Login                       `json:"login,omitempty"`
}

type ContainerCheckIgnoreUpgrade []string

type DiskUsage struct {
	DockerEnvName string           `json:"dockerEnvName"`
	Usage         *types.DiskUsage `json:"usage,omitempty"`
	UpdatedAt     time.Time        `json:"updatedAt,omitempty"`
}

type TwoFa struct {
	Secret string `json:"secret,omitempty"`
	Enable bool   `json:"enable,omitempty"`
	Email  string `json:"email,omitempty"`
	QrCode string `json:"qrCode,omitempty"`
}

type ThemeConfig map[string]any
type ThemeConsoleConfig map[string]any

type ThemeUserConfig struct {
	Token         map[string]interface{} `json:"token"`
	Components    map[string]interface{} `json:"components"`
	BgImage       ThemeUserConfigBgImage `json:"bgImage,omitempty"`
	SiteLink      []map[string]string    `json:"siteLink,omitempty"`
	SiteCopyright string                 `json:"siteCopyright,omitempty"`
	SiteTitle     string                 `json:"siteTitle"`
	SiteLogo      string                 `json:"siteLogo,omitempty"`
	LoginLogo     string                 `json:"loginLogo,omitempty"`
}

type ThemeUserConfigBgImage struct {
	Src    string `json:"src,omitempty"`
	Width  string `json:"width,omitempty"`
	Height string `json:"height,omitempty"`
	Top    int    `json:"top,omitempty"`
	Left   int    `json:"left,omitempty"`
	Right  int    `json:"right,omitempty"`
	Bottom int    `json:"bottom,omitempty"`
}

type DnsApi struct {
	Title      string           `json:"title,omitempty"`
	ServerName string           `json:"serverName,omitempty"`
	Env        []types2.EnvItem `json:"env,omitempty"`
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
	Group           string    `json:"group"`
	Item            []TagItem `json:"item,omitempty"`
}

type Login struct {
	FailedEnable        bool     `json:"failedEnable,omitempty"`
	FailedTotal         int      `json:"failedTotal,omitempty"`
	FailedLockTime      int      `json:"failedLockTime,omitempty"`
	NoPasswordEnable    bool     `json:"noPasswordEnable,omitempty"`
	NoPasswordUrl       string   `json:"noPasswordUrl,omitempty"`
	NoPasswordUserAgent string   `json:"noPasswordUserAgent,omitempty"`
	NoPasswordIpList    []string `json:"noPasswordIpList,omitempty"`
	DefaultRedirect     string   `json:"defaultRedirect,omitempty"`
}

type NotificationEmailServer struct {
	Host  string `json:"host,omitempty" binding:"required"`
	Port  int    `json:"port,omitempty" binding:"required"`
	Email string `json:"email,omitempty" binding:"required"`
	Code  string `json:"code,omitempty" binding:"required"`
}

type Notification struct {
	EmailServer *NotificationEmailServer `json:"emailServer,omitempty"`
	Status      []string                 `json:"status,omitempty"`
}
