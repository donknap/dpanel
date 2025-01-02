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
	RequestTimeout     int                       `json:"requestTimeout,omitempty"`
	Docker             map[string]*docker.Client `json:"docker,omitempty"`
	DiskUsage          *DiskUsage                `json:"diskUsage,omitempty"`
	TwoFa              *TwoFa                    `json:"twoFa,omitempty"`
	IgnoreCheckUpgrade []string                  `json:"ignoreCheckUpgrade,omitempty"`
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
