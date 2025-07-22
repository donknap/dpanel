package define

const (
	SwarmLabelService              = "com.docker.swarm.service.name"
	SwarmLabelServiceDescription   = "com.dpanel.swarm.service.description"
	SwarmLabelServiceImageRegistry = "com.dpanel.swarm.service.registry"
	SwarmLabelServiceVersion       = "com.dpanel.swarm.service.version" // 用于强制重建服务
	SwarmServiceModeGlobal         = "global"
	SwarmServiceModeReplicated     = "replicated"
)
