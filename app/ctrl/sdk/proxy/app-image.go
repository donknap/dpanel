package proxy

import (
	"encoding/json"
	"errors"

	"github.com/docker/docker/api/types/container"
	"github.com/donknap/dpanel/app/ctrl/sdk/types/app"
	"github.com/donknap/dpanel/common/function"
	"github.com/gin-gonic/gin"
)

// AppImageCheckUpgrade checks whether the image used by a container can be upgraded.
// Deprecated: use AppContainerCheckUpgrade instead.
func (self *Client) AppImageCheckUpgrade(params *app.ImageCheckUpgradeOption) (result app.ImageCheckUpgradeResult, err error) {
	data, err := self.Post(function.RouterApiUri("/app/container/get-list"), gin.H{
		"image": params.Md5,
	})
	if err != nil {
		return result, err
	}
	containerList := struct {
		List []container.Summary `json:"list"`
	}{}
	if err = json.NewDecoder(data).Decode(&containerList); err != nil {
		return result, err
	}
	containerID := ""
	for _, item := range containerList.List {
		if item.ImageID == params.Md5 && item.Image == params.Tag {
			containerID = item.ID
			break
		}
	}
	if containerID == "" {
		return result, errors.New("no container found using the specified image")
	}
	containerInfo, err := self.AppContainerGetDetail(containerID)
	if err != nil {
		return result, err
	}
	checkResult, err := self.AppContainerCheckUpgrade(&app.ContainerCheckUpgradeOption{
		ContainerID: containerInfo.Info.ID,
		Force:       params.CacheTime <= 0,
	})
	if err != nil {
		return result, err
	}
	return app.ImageCheckUpgradeResult{
		Upgrade:     checkResult.Upgrade,
		Digest:      checkResult.Digest,
		DigestLocal: checkResult.DigestLocal,
	}, nil
}

func (self *Client) AppImageTagRemote(params *app.ImageTagRemoteOption) (result app.ImageTagRemoteResult, err error) {
	data, err := self.Post(function.RouterApiUri("/app/image/tag-sync"), params)
	if err != nil {
		return result, err
	}
	err = json.NewDecoder(data).Decode(&result)
	return result, err
}
