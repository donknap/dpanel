package dockerfs

import "github.com/donknap/dpanel/common/service/docker"

type Option func(self *Fs) error

func WithTargetContainer(name string) Option {
	return func(self *Fs) error {
		self.targetContainerName = name
		self.targetType = targetTypeContainer
		return nil
	}
}

func WithMountTarget(root string) Option {
	return func(self *Fs) error {
		self.mountRoot = root
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
		if workingDir == "" {
			workingDir = "/"
		}
		self.workingDir = workingDir
		return nil
	}
}
