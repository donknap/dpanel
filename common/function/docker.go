package function

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/donknap/dpanel/common/service/docker/types"
	"github.com/donknap/dpanel/common/types/define"
)

func SplitCommandArray(cmd string) []string {
	result := make([]string, 0)
	field := ""
	ignoreSpace := false
	for _, s := range strings.Split(cmd, "") {
		if s == " " && !ignoreSpace {
			result = append(result, field)
			field = ""
			continue
		}
		if s == "\"" || s == "'" {
			ignoreSpace = !ignoreSpace
			continue
		}
		field += s
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

func ImageTag(tag string) *types.Tag {
	tag = strings.TrimPrefix(strings.TrimPrefix(tag, "http://"), "https://")
	result := &types.Tag{}

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
