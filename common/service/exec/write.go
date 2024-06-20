package exec

import "github.com/donknap/dpanel/common/service/docker"

type write struct {
}

func (self *write) Write(p []byte) (n int, err error) {
	docker.QueueDockerComposeMessage <- string(p)
	return len(p), nil
}
