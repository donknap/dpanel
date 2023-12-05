package accessor

import (
	"database/sql/driver"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/donknap/dpanel/common/service/docker"
)

type SiteContainerInfoOption struct {
	ID   string
	Info *types.Container
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
	sdk, err := docker.NewDockerClient()
	if err != nil {
		return nil
	}
	containerInfo, err := sdk.ContainerByField("id", id)
	if err != nil {
		return err
	}
	if item, ok := containerInfo[id]; ok {
		c.Info = item
	}
	c.ID = id
	return nil
}
