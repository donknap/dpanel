package types

import (
	"github.com/docker/docker/api/types/image"
	"github.com/donknap/dpanel/common/function"
)

type PermissionDataValue interface {
	GetPermissionDataValue(item any) interface{}
}

type ImgList []image.Summary

func (list ImgList) GetPermissionDataValue(item any) interface{} {
	return function.ImageTag(item.(image.Summary).RepoTags[0]).Registry
}
