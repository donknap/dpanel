package accessor

import (
	"html/template"
)

type SiteDomainSettingOption struct {
	ServerName                string        `json:"serverName"`
	ServerAddress             string        `json:"serverAddress"`
	TargetName                string        `json:"targetName"`
	Port                      int32         `json:"port"`
	EnableBlockCommonExploits bool          `json:"enableBlockCommonExploits"`
	EnableAssetCache          bool          `json:"enableAssetCache"`
	EnableWs                  bool          `json:"enableWs"`
	EnableSSL                 bool          `json:"enableSSL"`
	ExtraNginx                template.HTML `json:"extraNginx"`
	Email                     string        `json:"email"`
	SslCrt                    string        `json:"sslCrt"`
	SslKey                    string        `json:"sslKey"`
	SslCrtRenewTime           string        `json:"sslCrtRenewTime"`
	SslCrtCreaeTime           string        `json:"sslCrtCreaeTime"`
	SslCrtKey                 string        `json:"sslCrtKey"` // 标记当前域名获取证书信息的名称 acme.sh --info -d SslCrtKey
	AutoSsl                   bool          `json:"autoSsl"`
}
