package container

import (
	"github.com/donknap/dpanel/app/ctrl/sdk/proxy"
	"github.com/donknap/dpanel/app/ctrl/sdk/types/app"
	"github.com/donknap/dpanel/app/ctrl/sdk/utils"
	"github.com/donknap/dpanel/common/function"
	"github.com/spf13/cobra"
	"github.com/we7coreteam/w7-rangine-go/v2/src/console"
)

type Backup struct {
	console.Abstract
}

func (self Backup) GetName() string {
	return "container:backup"
}

func (self Backup) GetDescription() string {
	return "Create a container backup snapshot"
}

func (self Backup) Configure(command *cobra.Command) {
	command.Flags().String("docker-env", "local", "Docker server name")
	command.Flags().String("name", "", "Name of the container to backup")
	command.Flags().Bool("enable-image", false, "Enable container image backup")
	command.Flags().String("backup-image", "", "Backup image type: 'image' (registry image) or 'container' (commit container)")
	command.Flags().Bool("enable-volume", false, "Enable backup of mounted volumes")
	command.Flags().StringArray("backup-volume", []string{}, "Specific volume mount to backup")
	_ = command.MarkFlagRequired("name")
}

func (self Backup) Handle(cmd *cobra.Command, args []string) {
	name, _ := cmd.Flags().GetString("name")
	dockerEnv, _ := cmd.Flags().GetString("docker-env")
	enableImage, _ := cmd.Flags().GetBool("enable-image")
	backupImage, _ := cmd.Flags().GetString("backup-image")
	enableVolume, _ := cmd.Flags().GetBool("enable-volume")
	backupVolumeList, _ := cmd.Flags().GetStringArray("backup-volume")

	proxyClient, err := proxy.NewProxyClient()
	if err != nil {
		utils.Result{}.Error(err)
		return
	}
	dockerEnvList, err := proxyClient.CommonEnvGetList()
	if err != nil {
		utils.Result{}.Error(err)
		return
	}
	if dockerEnv != "" && dockerEnv != dockerEnvList.CurrentName {
		err = proxyClient.CommonEnvSwitch(dockerEnv)
		if err != nil {
			utils.Result{}.Error(err)
			return
		}
		defer func() {
			_ = proxyClient.CommonEnvSwitch(dockerEnvList.CurrentName)
		}()
	}
	params := &app.ContainerBackupOption{
		Id:           name,
		BackupVolume: "none",
		BackupImage:  "none",
	}
	if enableImage {
		params.BackupImage = "image"
	}
	if backupImage != "" {
		params.BackupImage = backupImage
	}
	if enableVolume {
		params.BackupVolume = "all"
	}
	if !function.IsEmptyArray(backupVolumeList) {
		params.BackupVolumeList = backupVolumeList
	}
	result, err := proxyClient.AppContainerBackupCreate(params)
	if err != nil {
		utils.Result{}.Error(err)
		return
	}
	utils.Result{}.Success(result)
	return
}
