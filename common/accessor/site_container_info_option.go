package accessor

import (
	"database/sql/driver"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/donknap/dpanel/common/service/docker"
)

const (
	StatusError   = 20 // 有错误
	StatusSuccess = 30 // 部署成功
)

type SiteContainerInfoOption struct {
	ID     string
	Info   *types.ContainerJSON
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
		c.Status = StatusError
		return nil
	}
	containerInfo, err := docker.Sdk.Client.ContainerInspect(docker.Sdk.Ctx, id)
	if err != nil {
		// 这里容器发生错误
		c.Err = err.Error()
		c.Status = StatusError
		return nil
	}
	if containerInfo.ID != "" {
		c.Info = &containerInfo

		if containerInfo.State.Running || containerInfo.State.Paused {
			c.Status = StatusSuccess
		} else {
			c.Status = StatusError
			c.Err = containerInfo.State.Status
		}

	} else {
		c.Err = "container not found"
		c.Status = StatusError
	}
	c.ID = id
	return nil
}
