package accessor

import (
	"html/template"
)

type SiteDomainSettingOption struct {
	ServerName                string        `json:"serverName" binding:"required"`
	ServerNameAlias           []string      `json:"serverNameAlias,omitempty"`
	ServerAddress             string        `json:"serverAddress"`
	ServerProtocol            string        `json:"serverProtocol"`
	ServerPort                string        `json:"serverPort"`
	TargetName                string        `json:"targetName"`
	Port                      int32         `json:"port" binding:"required"` // 目标转发转发端口 TargetPort
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
