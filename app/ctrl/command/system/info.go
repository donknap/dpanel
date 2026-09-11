package system

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/app/ctrl/sdk/proxy"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/function"
	"github.com/spf13/cobra"
)

type Info struct{}

const dpanelLogo = "██████╗ ██████╗  █████╗ ███╗   ██╗███████╗██╗     \n" +
	"██╔══██╗██╔══██╗██╔══██╗████╗  ██║██╔════╝██║     \n" +
	"██║  ██║██████╔╝███████║██╔██╗ ██║█████╗  ██║     \n" +
	"██║  ██║██╔═══╝ ██╔══██║██║╚██╗██║██╔══╝  ██║     \n" +
	"██████╔╝██║     ██║  ██║██║ ╚████║███████╗███████╗\n" +
	"╚═════╝ ╚═╝     ╚═╝  ╚═╝╚═╝  ╚═══╝╚══════╝╚══════╝\n"

type homeInfoResponse struct {
	DPanel  systemInfo  `json:"dpanel"`
	Founder founderInfo `json:"founder"`
}

type founderInfo struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type systemInfo struct {
	Name           string                   `json:"name"`
	Version        string                   `json:"version"`
	Family         string                   `json:"family"`
	Environment    string                   `json:"env"`
	RunIn          string                   `json:"runIn"`
	ServerHost     string                   `json:"serverHost"`
	ServerPort     int                      `json:"serverPort"`
	BaseURL        string                   `json:"baseUrl"`
	StoragePath    string                   `json:"storageLocalPath"`
	DNS            string                   `json:"dns"`
	Proxy          string                   `json:"proxy"`
	NoProxy        string                   `json:"noProxy"`
	SystemEntrance *accessor.SystemEntrance `json:"systemEntrance"`
}

func (self Info) GetName() string {
	return "system:info"
}

func (self Info) GetDescription() string {
	return "Display current system information"
}

func (self Info) Configure(command *cobra.Command) {
}

func (self Info) Handle(cmd *cobra.Command, args []string) {
	if _, err := (logic.Setting{}).GetValue(logic.SettingGroupUser, logic.SettingGroupUserFounder); err != nil {
		self.printInfo(self.localInfo(), false, true)
		return
	}

	client, err := proxy.NewProxyClient()
	if err != nil {
		self.printInfo(self.localInfo(), false, false)
		return
	}
	data, err := client.CommonHomeInfo()
	if err != nil {
		self.printInfo(self.localInfo(), false, false)
		return
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		self.printInfo(self.localInfo(), false, false)
		return
	}
	result := homeInfoResponse{}
	if err = json.Unmarshal(encoded, &result); err != nil {
		self.printInfo(self.localInfo(), false, false)
		return
	}
	self.printInfo(result, true, false)
}

func (self Info) localInfo() homeInfoResponse {
	dpanel := logic.Setting{}.GetDPanelInfo()
	login := logic.Setting{}.GetLoginSetting()
	return homeInfoResponse{
		DPanel: systemInfo{
			Name:           dpanel.Name,
			Version:        dpanel.Version,
			Family:         dpanel.Family,
			Environment:    dpanel.Env,
			RunIn:          dpanel.RunIn,
			ServerHost:     dpanel.ServerHost,
			ServerPort:     dpanel.ServerPort,
			BaseURL:        dpanel.BaseURL,
			StoragePath:    dpanel.StorageLocalPath,
			DNS:            dpanel.Dns,
			Proxy:          dpanel.Proxy,
			NoProxy:        dpanel.NoProxy,
			SystemEntrance: login.SystemEntrance,
		},
	}
}

func (self Info) printInfo(result homeInfoResponse, detailed bool, administratorRequired bool) {
	fmt.Println()
	fmt.Print(dpanelLogo)
	fmt.Println()

	writer := tabwriter.NewWriter(os.Stdout, 0, 0, 10, ' ', 0)
	_, _ = fmt.Fprintln(writer, "Name:\t", result.DPanel.Name)
	_, _ = fmt.Fprintln(writer, "Version:\t", result.DPanel.Version)
	_, _ = fmt.Fprintln(writer, "Family:\t", result.DPanel.Family)
	_, _ = fmt.Fprintln(writer, "Environment:\t", result.DPanel.Environment)
	if detailed {
		_, _ = fmt.Fprintln(writer, "Run In:\t", result.DPanel.RunIn)
	}
	_, _ = fmt.Fprintln(writer, "Server Address:\t", result.DPanel.ServerHost)
	_, _ = fmt.Fprintln(writer, "Server Port:\t", result.DPanel.ServerPort)
	_, _ = fmt.Fprintln(writer, "Base URL:\t", result.DPanel.BaseURL)
	_, _ = fmt.Fprintln(writer, "Storage Path:\t", result.DPanel.StoragePath)
	if detailed {
		_, _ = fmt.Fprintln(writer, "DNS:\t", result.DPanel.DNS)
		_, _ = fmt.Fprintln(writer, "Proxy:\t", function.MaskSensitiveValue(result.DPanel.Proxy))
		_, _ = fmt.Fprintln(writer, "No Proxy:\t", result.DPanel.NoProxy)
	}
	securityEntrance := ""
	if result.DPanel.SystemEntrance != nil && result.DPanel.SystemEntrance.Enable {
		securityEntrance = result.DPanel.SystemEntrance.Config
		if result.DPanel.SystemEntrance.Entrance != nil {
			securityEntrance = *result.DPanel.SystemEntrance.Entrance
		}
	}
	_, _ = fmt.Fprintln(writer, "Security Entrance:\t", securityEntrance)
	if administratorRequired {
		_, _ = fmt.Fprintln(writer, "Administrator:\t", "Not created. Please create an administrator account immediately.")
	} else if detailed {
		_, _ = fmt.Fprintln(writer, "Username:\t", result.Founder.Username)
		_, _ = fmt.Fprintln(writer, "Password (masked):\t", result.Founder.Password)
	}
	_, _ = fmt.Fprintln(writer, "Official Website:\t", "https://dpanel.cc")
	_, _ = fmt.Fprintln(writer, "GitHub Repository:\t", "https://github.com/donknap/dpanel")
	_ = writer.Flush()
}
