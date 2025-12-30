package function

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/donknap/dpanel/common/types/define"
)

func SplitCommandArray(cmd string) []string {
	result := make([]string, 0)
	field := ""
	// quoteChar 记录当前开启的引号类型 (单引 或 双引)
	// 只有当它为 0 时，空格才起分隔作用
	var quoteChar rune

	// 将字符串转为 rune 数组，处理中文字符更安全
	runes := []rune(cmd)

	for i := 0; i < len(runes); i++ {
		char := runes[i]

		if quoteChar == 0 {
			// 当前不在引号内
			if char == ' ' {
				if field != "" {
					result = append(result, field)
					field = ""
				}
				continue
			}
			if char == '"' || char == '\'' {
				// 开启引号模式
				quoteChar = char
				continue
			}
		} else {
			// 当前在引号内
			if char == quoteChar {
				// 遇到配对的引号，结束模式
				quoteChar = 0
				continue
			}
			// 处理转义字符 (如 \")
			if char == '\\' && i+1 < len(runes) && rune(runes[i+1]) == quoteChar {
				field += string(runes[i+1])
				i++ // 跳过下一个
				continue
			}
		}
		field += string(char)
	}

	if field != "" {
		result = append(result, field)
	}
	return result
}

func SplitStdout(reader io.Reader) (stdout bytes.Buffer, stderr bytes.Buffer, err error) {
	newReader := bufio.NewReader(reader)
	_, err = stdcopy.StdCopy(&stdout, &stderr, newReader)
	if err != nil {
		return stdout, stderr, err
	}
	return stdout, stderr, nil
}

func CombinedStdout(reader io.Reader) (out bytes.Buffer, err error) {
	newReader := bufio.NewReader(reader)
	_, err = stdcopy.StdCopy(&out, &out, newReader)
	if err != nil {
		return out, err
	}
	return out, nil
}

func ParseRestartPolicy(restartType string) (mode container.RestartPolicyMode) {
	restartPolicyMap := map[string]container.RestartPolicyMode{
		"always":         container.RestartPolicyAlways,
		"no":             container.RestartPolicyDisabled,
		"unless-stopped": container.RestartPolicyUnlessStopped,
		"on-failure":     container.RestartPolicyOnFailure,
	}
	if mode, ok := restartPolicyMap[restartType]; ok {
		return mode
	} else {
		return container.RestartPolicyDisabled
	}
}

func DefaultCapabilities() []string {
	return []string{
		"CAP_CHOWN",
		"CAP_DAC_OVERRIDE",
		"CAP_FSETID",
		"CAP_FOWNER",
		"CAP_MKNOD",
		"CAP_NET_RAW",
		"CAP_SETGID",
		"CAP_SETUID",
		"CAP_SETFCAP",
		"CAP_SETPCAP",
		"CAP_NET_BIND_SERVICE",
		"CAP_SYS_CHROOT",
		"CAP_KILL",
		"CAP_AUDIT_WRITE",
	}
}

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

func ImageTag(tag string) *Tag {
	tag = strings.TrimPrefix(strings.TrimPrefix(tag, "http://"), "https://")
	result := &Tag{}

	// 如果没有指定仓库地址，则默认为 docker.io
	noRegistryUrl := false
	temp := strings.Split(tag, "/")
	if !strings.Contains(temp[0], ".") || len(temp) == 1 {
		noRegistryUrl = true
		tag = define.RegistryDefaultName + "/" + tag
	}
	temp = strings.Split(tag, "/")
	// 先补齐 registry 地址后再判断是否有标签，仓库地址中可能包含端口号
	if !strings.Contains(strings.Join(temp[1:], "/"), ":") {
		tag += ":latest"
	}
	temp = strings.Split(tag, "/")
	result.Registry = temp[0]

	name := strings.Split(temp[len(temp)-1], ":")
	result.ImageName, result.Version = name[0], strings.Join(name[1:], ":")

	// 兼容使用 digest 标识版本号的情况
	if strings.Contains(result.Version, "@") {
		//result.Version = strings.Split(result.Version, "@")[1]
	}

	if len(temp) <= 2 {
		if noRegistryUrl {
			result.Namespace = "library"
		}
	} else {
		result.Namespace = strings.Join(temp[1:len(temp)-1], "/")
	}
	if result.Namespace != "" {
		result.BaseName = fmt.Sprintf("%s/%s", result.Namespace, result.ImageName)
	} else {
		result.BaseName = result.ImageName
	}

	return result
}
