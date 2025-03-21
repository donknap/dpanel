package store

import (
	"errors"
	"github.com/donknap/dpanel/app/ctrl/logic"
	"github.com/donknap/dpanel/common/dao"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
	"github.com/we7coreteam/w7-rangine-go/v2/src/console"
	"time"
)

type Sync struct {
	console.Abstract
}

func (self Sync) GetName() string {
	return "store:sync"
}

func (self Sync) GetDescription() string {
	return "同步应用商店数据"
}

func (self Sync) Configure(command *cobra.Command) {
	command.Flags().String("name", "", "商店标识")
	_ = command.MarkFlagRequired("name")
}

func (self Sync) Handle(cmd *cobra.Command, args []string) {
	name, _ := cmd.Flags().GetString("name")
	code, err := logic.User{}.GetAuth(time.Now().Add(time.Minute))
	if err != nil {
		logic.Result{}.Error(err)
		return
	}
	store, _ := dao.Store.Where(dao.Store.Name.Eq(name)).First()
	if store == nil {
		logic.Result{}.Error(errors.New("商店不存在，请先添加"))
		return
	}
	out, _, err := logic.Proxy{}.Post("/api/common/store/sync", code, gin.H{
		"id":   store.ID,
		"name": store.Name,
		"type": store.Setting.Type,
		"url":  store.Setting.Url,
	})
	if err != nil {
		logic.Result{}.Error(err)
		return
	}
	appList := out.Data.(map[string]interface{})
	if _, ok := appList["list"]; ok {
		logic.Result{}.Success(gin.H{
			"total": len(appList["list"].([]interface{})),
		})
		return
	}
	logic.Result{}.Error(errors.New("同步失败"))
	return
}
