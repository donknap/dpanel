package system

import (
	"github.com/donknap/dpanel/app/ctrl/sdk/proxy"
	"github.com/donknap/dpanel/app/ctrl/sdk/types/common"
	"github.com/donknap/dpanel/app/ctrl/sdk/utils"
	"github.com/spf13/cobra"
)

type Prune struct {
}

func (self Prune) GetName() string {
	return "system:prune"
}

func (self Prune) GetDescription() string {
	return "Cleanup temporary files, notices, events, and force-release memory"
}

func (self Prune) Configure(cmd *cobra.Command) {
	cmd.Flags().Bool("enable-notice", false, "Cleanup notices and events")
	cmd.Flags().Bool("enable-temp-file", false, "Cleanup temporary files")
}

func (self Prune) Handle(cmd *cobra.Command, args []string) {
	enableNotice, _ := cmd.Flags().GetBool("enable-notice")
	enableTempFile, _ := cmd.Flags().GetBool("enable-temp-file")

	proxyClient, err := proxy.NewProxyClient()
	if err != nil {
		utils.Result{}.Error(err)
		return
	}

	result, err := proxyClient.CommonPrune(common.PruneOption{
		EnableNotice:   enableNotice,
		EnableTempFile: enableTempFile,
	})
	if err != nil {
		utils.Result{}.Error(err)
		return
	}
	utils.Result{}.Success(result)
}
