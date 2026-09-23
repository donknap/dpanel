package system

import (
	"path/filepath"

	"github.com/donknap/dpanel/app/ctrl/sdk/proxy"
	"github.com/donknap/dpanel/app/ctrl/sdk/types/common"
	"github.com/donknap/dpanel/app/ctrl/sdk/utils"
	"github.com/spf13/cobra"
)

type Backup struct {
}

func (self Backup) GetName() string {
	return "system:backup"
}

func (self Backup) GetDescription() string {
	return "Back up DPanel data"
}

func (self Backup) Configure(cmd *cobra.Command) {
	cmd.Flags().StringArray("backup-path", []string{}, "Panel data path to back up; use an exact panel path value such as ./dpanel.db; leave empty to back up all panel data")
	cmd.Flags().StringArray("ignore-path-prefix", []string{}, "Directory prefix to exclude from the backup")
}

func (self Backup) Handle(cmd *cobra.Command, args []string) {
	backupPathList, _ := cmd.Flags().GetStringArray("backup-path")
	ignorePathPrefix, _ := cmd.Flags().GetStringArray("ignore-path-prefix")

	proxyClient, err := proxy.NewProxyClient()
	if err != nil {
		utils.Result{}.Error(err)
		return
	}

	result, err := proxyClient.CommonPanelBackup(common.PanelBackupOption{
		BackupPathList:         backupPathList,
		IgnoreVolumePathPrefix: ignorePathPrefix,
	})
	if err != nil {
		utils.Result{}.Error(err)
		return
	}
	result.Path = filepath.Base(result.Path)
	utils.Result{}.Success(result)
}
