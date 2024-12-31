package accessor

import (
	"github.com/docker/docker/api/types"
	"github.com/donknap/dpanel/common/service/docker"
	"time"
)

type SettingValueOption struct {
	Username         string                    `json:"username,omitempty"`
	Password         string                    `json:"password,omitempty"`
	ServerIp         string                    `json:"serverIp,omitempty"`
	RequestTimeout   int                       `json:"requestTimeout,omitempty"`
	Docker           map[string]*docker.Client `json:"docker,omitempty"`
	DiskUsage        *DiskUsage                `json:"diskUsage,omitempty"`
	TwoFa            *TwoFa                    `json:"twoFa,omitempty"`
	ContainerUpgrade *CheckContainerUpgrade    `json:"containerUpgrade,omitempty"`
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

type CheckContainerUpgrade struct {
	ExpireTime    time.Time `json:"expreTime,omitempty"`
	Upgrade       bool      `json:"upgrade,omitempty"`
	Digest        string    `json:"digest,omitempty"`
	IgnoreDigest  string    `json:"ignoreDigest,omitempty"`  // 忽略本次
	IgnoreUpgrade bool      `json:"ignoreUpgrade,omitempty"` // 永久忽略
}
