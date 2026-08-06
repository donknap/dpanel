package types

import (
	"strings"

	registryTypes "github.com/we7coreteam/registry-go-sdk/types"
)

type ImageDigestInspectOption struct {
	RegistryAddresses  []string
	RegistryCredential registryTypes.Credential
}

type ImageDigestInspectResult struct {
	ImageID      string
	ImageName    string
	LocalDigests []string
	RemoteDigest string
}

func (self ImageDigestInspectResult) IsAvailable() bool {
	return len(self.LocalDigests) > 0 && self.RemoteDigest != ""
}

func (self ImageDigestInspectResult) IsDifferent() bool {
	if !self.IsAvailable() {
		return false
	}
	for _, localDigest := range self.LocalDigests {
		if strings.HasSuffix(localDigest, self.RemoteDigest) {
			return false
		}
	}
	return true
}
