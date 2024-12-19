package store

import (
	"github.com/donknap/dpanel/app/ctrl/logic"
	"github.com/donknap/dpanel/common/dao"
	"github.com/gin-gonic/gin"
	"github.com/gookit/color"
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
		color.Error.Println(err)
		return
	}
	store, _ := dao.Store.Where(dao.Store.Name.Eq(name)).First()
	if store == nil {
		color.Error.Println("商店不存在，请先添加")
		return
	}
	out, err := logic.Proxy{}.Post("/api/common/store/sync", code, gin.H{
		"id":   store.ID,
		"name": store.Name,
		"type": store.Setting.Type,
		"url":  store.Setting.Url,
	})
	if err != nil {
		color.Error.Println(err)
		return
	}
	appList := out.Data.(map[string]interface{})
	if _, ok := appList["list"]; ok {
		color.Successln("商店数据同步成功", "共同步了", len(appList["list"].([]interface{})), "条应用数据")
	} else {
		color.Successln("商店数据同步成功")
	}
}
