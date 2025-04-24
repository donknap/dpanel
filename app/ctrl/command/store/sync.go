package store

import (
	"github.com/donknap/dpanel/app/ctrl/sdk/proxy"
	"github.com/donknap/dpanel/app/ctrl/sdk/types/common"
	"github.com/donknap/dpanel/app/ctrl/sdk/utils"
	"github.com/donknap/dpanel/common/dao"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
	"github.com/we7coreteam/w7-rangine-go/v2/src/console"
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

	store, _ := dao.Store.Where(dao.Store.Name.Eq(name)).First()
	if store == nil {
		utils.Result{}.Errorf("%s 商店不存在，请先添加", name)
		return
	}
	proxyClient, err := proxy.NewProxyClient()
	if err != nil {
		utils.Result{}.Error(err)
		return
	}

	appList, err := proxyClient.CommonStoreSync(&common.StoreSyncOption{
		Id:   store.ID,
		Name: store.Name,
		Type: store.Setting.Type,
		Url:  store.Setting.Url,
	})
	if err != nil {
		utils.Result{}.Error(err)
		return
	}

	utils.Result{}.Success(gin.H{
		"total": len(appList),
	})
	return
}
