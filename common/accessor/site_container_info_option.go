package accessor

import (
	"database/sql/driver"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/donknap/dpanel/common/service/docker"
)

const (
	STATUS_ERROR   = 20 // 有错误
	STATUS_SUCCESS = 30 // 部署成功
)

type SiteContainerInfoOption struct {
	ID     string
	Info   *types.Container
	Err    string
	Status int32
}

func (c SiteContainerInfoOption) Value() (driver.Value, error) {
	// 只需要保存容器id即可，获取时通过接口拿到真实的数据
	return c.ID, nil
}

func (c *SiteContainerInfoOption) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	id, ok := value.(string)
	if !ok {
		return fmt.Errorf("value is not string id, value: %v", value)
	}
	if id == "" {
		c.Err = "container not found"
		c.Status = STATUS_ERROR
		return nil
	}
	containerInfo, err := docker.Sdk.ContainerByField("id", id)
	if err != nil {
		// 这里容器发生错误
		c.Err = err.Error()
		c.Status = STATUS_ERROR
		return nil
	}
	if item, ok := containerInfo[id]; ok {
		c.Info = item
		if item.State == "running" || item.State == "paused" {
			c.Status = STATUS_SUCCESS
		} else {
			c.Status = STATUS_ERROR
			c.Err = item.Status
		}

	} else {
		c.Err = "container not found"
		c.Status = STATUS_ERROR
	}
	c.ID = id
	return nil
}
