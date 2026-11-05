package sync

import (
	"strings"

	"github.com/distribution/reference"
)

// IsZip 仅按地址后缀判断，不请求远端确认文件内容。
func IsZip(source string) bool {
	address, _, _ := strings.Cut(source, "#")
	address, _, _ = strings.Cut(address, "?")
	return strings.HasSuffix(strings.ToLower(address), ".zip")
}

// IsGit 使用显式 .git 后缀，允许通过 # 指定分支。
func IsGit(source string) bool {
	address, _, _ := strings.Cut(source, "#")
	return strings.HasSuffix(address, ".git")
}

func IsOci(source string) bool {
	_, err := reference.ParseNormalizedNamed(source)
	return err == nil
}
