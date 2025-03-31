package accessor

import (
	"html/template"
)

type SiteDomainSettingOption struct {
	ServerName                string        `json:"serverName"`
	ServerNameAlias           []string      `json:"serverNameAlias,omitempty"`
	ServerAddress             string        `json:"serverAddress"`
	ServerProtocol            string        `json:"serverProtocol"`
	TargetName                string        `json:"targetName"`
	Port                      int32         `json:"port"`
	EnableBlockCommonExploits bool          `json:"enableBlockCommonExploits"`
	EnableAssetCache          bool          `json:"enableAssetCache"`
	EnableWs                  bool          `json:"enableWs"`
	EnableSSL                 bool          `json:"enableSSL"`
	ExtraNginx                template.HTML `json:"extraNginx,omitempty"`
	Email                     string        `json:"email,omitempty"`
	SslCrt                    string        `json:"sslCrt,omitempty"`
	SslKey                    string        `json:"sslKey,omitempty"`
	CertName                  string        `json:"certName,omitempty"`
}
