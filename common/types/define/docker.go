package define

import "time"

const (
	DockerConnectServerTimeout = time.Second * 10
)

const (
	DockerContainerBackupTypeSnapshot = "snapshot"
)

const (
	DockerRemoteTypeSSH  = "ssh"
	DockerRemoteTypeSock = "sock"
	DockerRemoteTypeTcp  = "tcp"
)

const (
	DockerDefaultClientName = "local"
	DockerContextName       = "dpanel-context-%s"
	DockerBuilderName       = DockerContextName + "-builder"
)

const (
	DockerImageBuildStatusStop    = 0  // 未开始
	DockerImageBuildStatusProcess = 10 // 进行中
	DockerImageBuildStatusError   = 20 // 有错误
	DockerImageBuildStatusSuccess = 30 // 部署成功
)

const (
	DockerMessageTypeContainerStart   = "container/start"
	DockerMessageTypeContainerDie     = "container/die"
	DockerMessageTypeContainerCreate  = "container/create"
	DockerMessageTypeContainerDestroy = "container/destroy"
	DockerMessageTypeDaemonStart      = "daemon/start"
	DockerMessageTypeDaemonDie        = "daemon/die"
)

const (
	DockerAnnotationRefName = "io.containerd.image.name"
)
