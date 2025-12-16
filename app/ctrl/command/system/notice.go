package system

import (
	"github.com/donknap/dpanel/app/ctrl/sdk/proxy"
	"github.com/donknap/dpanel/app/ctrl/sdk/types/common"
	"github.com/donknap/dpanel/app/ctrl/sdk/utils"
	"github.com/spf13/cobra"
)

type Notice struct {
}

func (self Notice) GetName() string {
	return "system:notice"
}

func (self Notice) GetDescription() string {
	return "Send a notification"
}

func (self Notice) Configure(cmd *cobra.Command) {
	cmd.Flags().String("content", "", `The content of the notification`)
	cmd.Flags().String("subject", "", `The subject of the notification`)
	cmd.Flags().String("target", "", `The recipient of the notification; when using email, please enter the email address.`)
	cmd.Flags().String("channel", "email", `Channels for sending notifications ("email")`)
	_ = cmd.MarkFlagRequired("content")
	_ = cmd.MarkFlagRequired("target")
}

func (self Notice) Handle(cmd *cobra.Command, args []string) {
	content, _ := cmd.Flags().GetString("content")
	channel, _ := cmd.Flags().GetString("channel")
	subject, _ := cmd.Flags().GetString("subject")
	target, _ := cmd.Flags().GetString("target")

	proxyClient, err := proxy.NewProxyClient()
	if err != nil {
		utils.Result{}.Error(err)
		return
	}

	result, err := proxyClient.CommonNotification(common.NotificationOption{
		Channel: channel,
		Subject: subject,
		Content: content,
		Target:  target,
	})
	if err != nil {
		utils.Result{}.Error(err)
		return
	}
	utils.Result{}.Success(result)
	return
}
