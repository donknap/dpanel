package define

import "time"

const (
	DockerConnectServerTimeout = time.Second * 10
)

const (
	DockerContainerBackupTypeSnapshot = "snapshot"
)

const (
	DockerRemoteTypeSSH    = "ssh"
	DockerRemoteTypeDocker = "docker"
)

const (
	DockerDefaultClientName = "local"
)

const (
	DockerImageBuildStatusStop    = 0  // 未开始
	DockerImageBuildStatusProcess = 10 // 进行中
	DockerImageBuildStatusError   = 20 // 有错误
	DockerImageBuildStatusSuccess = 30 // 部署成功
)
