package controller

import (
	"github.com/donknap/dpanel/common/dao"
	"github.com/gin-gonic/gin"
)

func (self Volume) GetBackupList(http *gin.Context) {
	type ParamsValidate struct {
		ContainerId string `json:"containerId"`
		Page        int    `json:"page,default=1" binding:"omitempty,gt=0"`
		PageSize    int    `json:"pageSize" binding:"omitempty,gt=1"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 {
		params.PageSize = 10
	}

	query := dao.Backup.Order(dao.Backup.ID.Desc())
	if params.ContainerId != "" {
		query = query.Where(dao.Backup.ContainerID.Like("%" + params.ContainerId + "%"))
	}
	list, total, _ := query.FindByPage((params.Page-1)*params.PageSize, params.PageSize)
	self.JsonResponseWithoutError(http, gin.H{
		"total": total,
		"page":  params.Page,
		"list":  list,
	})
	return
}
