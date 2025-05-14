package dockerfs

import "github.com/donknap/dpanel/common/service/docker"

type Option func(self *Fs) error

func WithTargetContainer(name, root string) Option {
	return func(self *Fs) error {
		self.targetContainerRootPath = root
		self.targetContainerName = name
		return nil
	}
}

func WithProxyContainer(name string) Option {
	return func(self *Fs) error {
		self.proxyContainerName = name
		return nil
	}
}

func WithDockerSdk(sdk *docker.Builder) Option {
	return func(self *Fs) error {
		self.sdk = sdk
		return nil
	}
}
