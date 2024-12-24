package docker

import (
	"bytes"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
	"io"
	"strings"
)

func GetContentFromStdFormat(reader io.Reader) (*bytes.Buffer, error) {
	buffer := new(bytes.Buffer)
	_, err := io.Copy(buffer, reader)
	if err != nil {
		return nil, err
	}
	newReader := bytes.NewReader(buffer.Bytes())
	stdout := new(bytes.Buffer)
	_, err = stdcopy.StdCopy(stdout, stdout, newReader)
	if err == nil {
		return stdout, nil
	} else {
		return buffer, nil
	}
}

func GetRestartPolicyByString(restartType string) (mode container.RestartPolicyMode) {
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

func CommandSplit(cmd string) []string {
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

func NewValueItemFromMap(maps map[string]string) (r []ValueItem) {
	for name, value := range maps {
		r = append(r, ValueItem{
			Name:  name,
			Value: value,
		})
	}
	return r
}
