package system

import (
	"github.com/donknap/dpanel/app/ctrl/sdk/proxy"
	"github.com/donknap/dpanel/app/ctrl/sdk/types/common"
	"github.com/donknap/dpanel/app/ctrl/sdk/utils"
	"github.com/spf13/cobra"
)

type Cache struct {
}

func (self Cache) GetName() string {
	return "system:cache"
}

func (self Cache) GetDescription() string {
	return "Simple key/value cache to help store data in task scripts."
}

func (self Cache) Configure(cmd *cobra.Command) {
	cmd.Flags().String("key", "", "Cache name; data with the same name will be overwritten. eg: test")
	cmd.Flags().String("value", "", `Cached data, if empty return the saved value. eg: "test", "1", "hello world"`)
	cmd.Flags().Int("keep", -1, "The lifecycle of cached data, in seconds. Use -1 is until the DPanel progress exits")
	_ = cmd.MarkFlagRequired("key")
}

func (self Cache) Handle(cmd *cobra.Command, args []string) {
	key, _ := cmd.Flags().GetString("key")
	value, _ := cmd.Flags().GetString("value")
	keep, _ := cmd.Flags().GetInt("keep")

	proxyClient, err := proxy.NewProxyClient()
	if err != nil {
		utils.Result{}.Error(err)
		return
	}

	result, err := proxyClient.CommonCache(common.CacheOption{
		Key:   key,
		Value: value,
		Keep:  keep,
	})
	if err != nil {
		utils.Result{}.Error(err)
		return
	}
	utils.Result{}.Success(result)
}
