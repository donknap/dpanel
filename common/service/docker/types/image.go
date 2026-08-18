package types

import "strings"

type ImageDigestInspectOption struct {
	Registry Registry
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
