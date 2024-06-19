package accessor

import (
	"github.com/go-acme/lego/v4/certificate"
	"html/template"
)

type SiteDomainSettingOption struct {
	ServerName                string                `json:"serverName"`
	ServerAddress             string                `json:"serverAddress"`
	TargetName                string                `json:"targetName"`
	Port                      int32                 `json:"port"`
	EnableBlockCommonExploits bool                  `json:"enableBlockCommonExploits"`
	EnableAssetCache          bool                  `json:"enableAssetCache"`
	EnableWs                  bool                  `json:"enableWs"`
	EnableSSL                 bool                  `json:"enableSSL"`
	ExtraNginx                template.HTML         `json:"extraNginx"`
	Email                     string                `json:"email"`
	SslResource               *certificate.Resource `json:"sslResource"`
}
