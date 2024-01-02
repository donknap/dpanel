package accessor

import (
	"database/sql/driver"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/donknap/dpanel/common/service/docker"
	"log/slog"
)

type ImageInfoOption struct {
	Id   string             `json:"id"`
	Info types.ImageInspect `json:"info"`
}

func (c ImageInfoOption) Value() (driver.Value, error) {
	return c.Id, nil
}

func (c *ImageInfoOption) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	id, ok := value.(string)
	if !ok {
		return fmt.Errorf("value is not string id, value: %v", value)
	}
	if id == "" {
		slog.Debug("tag not found")
		return nil
	}
	c.Id = id
	imageInfo, _, err := docker.Sdk.Client.ImageInspectWithRaw(docker.Sdk.Ctx, id)
	if err != nil {
		slog.Debug(err.Error())
		return nil
	}
	c.Info = imageInfo
	return nil
}
