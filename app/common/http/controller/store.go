package controller

import (
	"errors"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/dao"
	"github.com/donknap/dpanel/common/entity"
	"github.com/donknap/dpanel/common/service/exec"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller"
	"io"
	"os"
	"path/filepath"
	"time"
)

type Store struct {
	controller.Abstract
}

func (self Store) Create(http *gin.Context) {
	type ParamsValidate struct {
		Id    int32  `json:"id"`
		Title string `json:"title" binding:"required"`
		Type  string `json:"type" binding:"required"`
		Name  string `json:"name" binding:"required"`
		Git   string `json:"git"`
		Url   string `json:"url"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	if params.Id <= 0 {
		storeRow, _ := dao.Store.Where(dao.Store.Name.Eq(params.Name)).First()
		if storeRow != nil {
			self.JsonResponseWithError(http, errors.New("应用商店已经存在"), 500)
			return
		}
	} else {
		storeRow, _ := dao.Store.Where(dao.Store.ID.Eq(params.Id)).First()
		if storeRow == nil {
			self.JsonResponseWithError(http, errors.New("应用商店不存在"), 500)
			return
		}
	}

	if params.Type == accessor.StoreType1Panel {
		err := os.RemoveAll(filepath.Join(storage.Local{}.GetStorePath(), params.Name))
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		out, err := exec.Command{}.Run(&exec.RunCommandOption{
			CmdName: "git",
			CmdArgs: []string{
				"clone", "--depth", "1",
				params.Git, filepath.Join(storage.Local{}.GetStorePath(), params.Name),
			},
			Timeout: time.Second * 30,
		})
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
		_, err = io.Copy(os.Stdout, out)
		if err != nil {
			self.JsonResponseWithError(http, err, 500)
			return
		}
	}

	storeNew := &entity.Store{
		Title: params.Title,
		Name:  params.Name,
		Setting: &accessor.StoreSettingOption{
			Type: params.Type,
			Git:  params.Git,
			Url:  params.Url,
		},
	}
	var err error
	if params.Id <= 0 {
		err = dao.Store.Create(storeNew)
	} else {
		_, err = dao.Store.Where(dao.Store.ID.Eq(params.Id)).Updates(storeNew)
		storeNew.ID = params.Id
	}

	if err != nil {
		self.JsonResponseWithError(http, err, 500)
		return
	}

	self.JsonSuccessResponse(http)
	return
}

func (self Store) Delete(http *gin.Context) {

}

func (self Store) GetList(http *gin.Context) {
	type ParamsValidate struct {
		Title string `json:"title"`
	}
	params := ParamsValidate{}
	if !self.Validate(http, &params) {
		return
	}

	var list []*entity.Store

	query := dao.Store.Order(dao.Store.ID.Desc())
	if params.Title != "" {
		query = query.Where(dao.Store.Title.Like("%" + params.Title + "%"))
	}
	list, _ = query.Find()
	self.JsonResponseWithoutError(http, gin.H{
		"list": list,
	})
	return
}

func (self Store) Update(http *gin.Context) {

}
