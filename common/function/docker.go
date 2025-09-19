package function

import (
	"bufio"
	"bytes"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
	"io"
	"strings"
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
