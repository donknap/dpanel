package logic

import (
	"context"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/service/docker"
)

type ImageLogic struct {
}

func (self ImageLogic) SyncImage() (err error) {
	sdk, _ := docker.NewDockerClient()
	list, err := sdk.Client.ImageList(context.Background(), types.ImageListOptions{
		All:            false,
		ContainerCount: true,
	})
	if err != nil {
		return err
	}
	if list != nil {
		for _, imageSummary := range list {
			query := dao.Image.Where(
				dao.Image.Md5.Eq(imageSummary.ID),
			)
			imageRow, _ := query.First()
			if imageRow != nil {
				query.Updates(entity.Image{
					Size: fmt.Sprintf("%d", imageSummary.Size),
					Tag: &accessor.ImageTagOption{
						Tag: imageSummary.RepoTags,
					},
					ContainerTotal: int32(imageSummary.Containers),
				})
			} else {
				if len(imageSummary.RepoTags) > 0 {
					rowNew := &entity.Image{
						Name: imageSummary.RepoTags[0],
						Md5:  imageSummary.ID,
						Size: fmt.Sprintf("%d", imageSummary.Size),
						Tag: &accessor.ImageTagOption{
							Tag: imageSummary.RepoTags,
						},
						ContainerTotal: int32(imageSummary.Containers),
						CreatedAt:      int32(imageSummary.Created),
						Status:         STATUS_SUCCESS,
						Type:           IMAGE_TYPE_ALL,
					}
					dao.Image.Create(rowNew)
				}
			}
		}
	}
	return err
}
