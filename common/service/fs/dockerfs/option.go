package dockerfs

import "github.com/donknap/dpanel/common/service/docker"

type Option func(self *Fs) error

func WithName(name string) Option {
	return func(self *Fs) error {
		self.name = name
		return nil
	}
}

func WithTargetContainer(name string) Option {
	return func(self *Fs) error {
		self.targetContainerName = name
		self.targetType = targetTypeContainer
		self.rootPath = "/"
		return nil
	}
}

func WithRoot(root string) Option {
	return func(self *Fs) error {
		self.rootPath = root
		self.targetType = targetTypeMount
		return nil
	}
}

func WithProxyContainer(name string) Option {
	return func(self *Fs) error {
		self.proxyContainerName = name
		return nil
	}
}

func WithDockerSdk(sdk *docker.Client) Option {
	return func(self *Fs) error {
		self.sdk = sdk
		return nil
	}
}

func WithWorkingDir(workingDir string) Option {
	return func(self *Fs) error {
		self.workingDir = workingDir
		return nil
	}
}
