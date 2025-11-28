package define

const (
	StoreTypeOnePanel      = "1panel"
	StoreTypeOnePanelLocal = "1panel-local"
	StoreTypePortainer     = "portainer"
	StoreTypeCasaOs        = "casaos"
	StoreTypeBaoTa         = "baota"
)

const (
	StoreTypeOnePanelNetwork = "1panel-network"
	StoreTypeBaoTaNetwork    = "baota_net"
)

const (
	_ = iota
	StoreEnvContainerName
	StoreEnvImageName
	StoreEnvWebsiteDir
	StoreEnvCodeDir
	StoreEnvHostIp
	StoreEnvHostPort
	StoreEnvContainerPort
	StoreEnvRunScript
	StoreEnvVersion
	StoreEnvLimitCpu
	StoreEnvLimitMemory
	StoreEnvDBHost
	StoreEnvDBPort
)
