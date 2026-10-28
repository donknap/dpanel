package types

import "strings"

type ImageDigestInspectOption struct {
	Registry        Registry
	LocalDigestList []string
}

type ImageDigestInspectResult struct {
	ImageID      string
	ImageName    string
	LocalDigests []string
	RemoteDigest *string // nil 表示未取得 manifest，空字符串表示 manifest 存在但未返回摘要。
	Error        string
}

func (self ImageDigestInspectResult) IsAvailable() bool {
	return self.RemoteDigest != nil && (*self.RemoteDigest != "" || len(self.LocalDigests) == 0)
}

func (self ImageDigestInspectResult) IsDifferent() bool {
	if !self.IsAvailable() {
		return false
	}
	for _, localDigest := range self.LocalDigests {
		if strings.HasSuffix(localDigest, *self.RemoteDigest) {
			return false
		}
	}
	return true
}
