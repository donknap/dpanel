package accessor

import (
	"fmt"
	"html/template"
)

const (
	VhostFileName        = "%s.conf"
	VhostDisableFilename = VhostFileName + ".disable"
)

type SiteDomainSettingOption struct {
	Title                     string        `json:"title,omitempty"` // 域名描述说明
	ServerName                string        `json:"serverName" binding:"required"`
	ServerNameAlias           []string      `json:"serverNameAlias,omitempty"`
	ServerAddress             string        `json:"serverAddress"`
	ServerProtocol            string        `json:"serverProtocol"`
	ServerPort                string        `json:"serverPort"`
	TargetName                string        `json:"targetName"`
	Port                      int32         `json:"port" binding:"required"` // 目标转发转发端口 TargetPort
	TargetPort                int32         `json:"targetPort"`
	EnableDisable             bool          `json:"enableDisable"` // 是否禁用, 会增加 .disable 后缀
	EnableBlockCommonExploits bool          `json:"enableBlockCommonExploits"`
	EnableAssetCache          bool          `json:"enableAssetCache"`
	EnableWs                  bool          `json:"enableWs"`
	EnableSSL                 bool          `json:"enableSSL"`
	ExtraNginx                template.HTML `json:"extraNginx,omitempty"`
	SslCrt                    string        `json:"sslCrt,omitempty"`
	SslKey                    string        `json:"sslKey,omitempty"`
	CertName                  string        `json:"certName,omitempty"`
	Type                      string        `json:"type"`
	WWWRoot                   string        `json:"wwwRoot,omitempty"`
	FPMRoot                   string        `json:"fpmRoot,omitempty"`
}

// VHostFilename 返回 vhost 配置文件名
// 当 Enable 为 false 时，返回 .disable 后缀的文件名以跳过 nginx 加载
func (s *SiteDomainSettingOption) VHostFilename() string {
	if s.EnableDisable {
		return fmt.Sprintf(VhostDisableFilename, s.ServerName)
	}
	return fmt.Sprintf(VhostFileName, s.ServerName)
}
