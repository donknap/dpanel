package function

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime"
	"strings"

	"github.com/distribution/reference"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/mattn/go-shellwords"
)

func SplitCommandArray(cmd string) []string {
	result := make([]string, 0)
	if runtime.GOOS == "windows" {
		// windows 中需要将路径再次转义一下，防止 parade 之后没有分隔符
		cmd = strings.ReplaceAll(cmd, "\\", "\\\\")
	}
	result, err := shellwords.Parse(cmd)
	if err != nil {
		slog.Debug("function split command array", "error", err)
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
	Name      string `json:"name"`
	Registry  string `json:"registry"`
	Namespace string `json:"namespace"`
	ImageName string `json:"imageName"`
	Version   string `json:"version"`
	BaseName  string `json:"baseName"`
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

func (self Tag) getName() string {
	version := self.Version
	if strings.Contains(self.Version, "@") {
		version = strings.Split(version, "@")[0]
	}
	if self.Namespace == "" {
		return fmt.Sprintf("%s:%s", self.ImageName, version)
	} else {
		return fmt.Sprintf("%s/%s:%s", self.Namespace, self.ImageName, version)
	}
}

func ImageTag(tag string) *Tag {
	tag = strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(tag, "/"), "http://"), "https://")
	result := &Tag{}

	ref, err := reference.ParseNormalizedNamed(tag)
	if err != nil {
		return result
	}
	result.Registry = reference.Domain(ref)
	result.BaseName = reference.Path(ref)

	// .String() docker.io/test/phpmyadmin:latest@sha256
	tagName := reference.TagNameOnly(ref)
	if i := strings.LastIndex(tagName.String(), result.BaseName); i > -1 {
		result.Version = tagName.String()[i+len(result.BaseName)+1:]
	}

	// 假如 basename 包含 / 再进一步的分割 namespace 和 imageName
	if i := strings.LastIndex(result.BaseName, "/"); i > -1 {
		result.Namespace = result.BaseName[:i]
		result.ImageName = result.BaseName[i+1:]
	} else {
		result.ImageName = result.BaseName
	}
	// 如果当前是 docker.io 没有 namespace 时默认添加上 library
	if result.Registry == "docker.io" && result.Namespace == "" {
		result.Namespace = "library"
	}
	result.Name = result.getName()
	return result
}

func IsRunInDocker() bool {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	if file, err := os.ReadFile("/proc/self/mountinfo"); err == nil {
		return strings.Contains(string(file), "docker") || strings.Contains(string(file), "containerd")
	}
	return false
}
