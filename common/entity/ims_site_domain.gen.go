// Code generated by gorm.io/gen. DO NOT EDIT.
// Code generated by gorm.io/gen. DO NOT EDIT.
// Code generated by gorm.io/gen. DO NOT EDIT.

package entity

import (
	"time"

	"github.com/donknap/dpanel/common/accessor"
)

const TableNameSiteDomain = "ims_site_domain"

// SiteDomain mapped from table <ims_site_domain>
type SiteDomain struct {
	ID          int32                             `gorm:"column:id;primaryKey" json:"id"`
	ContainerID string                            `gorm:"column:container_id" json:"containerId"`
	ServerName  string                            `gorm:"column:server_name" json:"serverName"`
	Setting     *accessor.SiteDomainSettingOption `gorm:"column:setting;serializer:json" json:"setting"`
	CreatedAt   time.Time                         `gorm:"column:created_at" json:"createdAt"`
}

// TableName SiteDomain's table name
func (*SiteDomain) TableName() string {
	return TableNameSiteDomain
}
