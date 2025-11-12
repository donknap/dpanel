package types

import (
	"fmt"
	"strings"

	"github.com/donknap/dpanel/common/types/define"
)

// Tag {registry}/{{namespace-可能有多个路径}/{imageName}basename}:{version}
type Tag struct {
	Registry  string
	Namespace string
	ImageName string
	Version   string
	BaseName  string
}

func (self Tag) Uri() string {
	if self.Registry == "" {
		self.Registry = define.RegistryDefaultName
	}
	self.Registry = strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(self.Registry, "http://"), "https://"), "/")
	split := ":"
	if self.Namespace == "" {
		return fmt.Sprintf("%s/%s%s%s", self.Registry, self.ImageName, split, self.Version)
	} else {
		return fmt.Sprintf("%s/%s/%s%s%s", self.Registry, self.Namespace, self.ImageName, split, self.Version)
	}
}

func (self Tag) Name() string {
	version := self.Version
	if strings.Contains(self.Version, "@") {
		version = strings.Split(version, "@")[0]
	}
	if self.Namespace == "" || self.Namespace == "library" {
		return fmt.Sprintf("%s:%s", self.ImageName, version)
	} else {
		return fmt.Sprintf("%s/%s:%s", self.Namespace, self.ImageName, version)
	}
}
