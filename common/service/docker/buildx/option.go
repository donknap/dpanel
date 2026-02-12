package buildx

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types/registry"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker/types"
	"github.com/donknap/dpanel/common/service/storage"
)

type Option func(self *Builder) error

func WithWorkDir(path string) Option {
	return func(self *Builder) error {
		self.options.WorkDir = path
		return nil
	}
}

// WithTag 添加镜像 Tag (支持多个)
func WithTag(targetName string, tags ...string) Option {
	return func(self *Builder) error {
		if len(tags) == 0 {
			return nil
		}

		found := false
		for i, group := range self.options.Target {
			if group.Target == targetName {
				self.options.Target[i].Tags = append(self.options.Target[i].Tags, tags...)
				found = true
				break
			}
		}

		if !found {
			self.options.Target = append(self.options.Target, BuildOptionsTarget{
				Target: targetName,
				Tags:   tags,
			})
		}
		return nil
	}
}

// WithDockerFilePath WithDockerFile 指定 Dockerfile 路径
func WithDockerFilePath(path string) Option {
	return func(self *Builder) error {
		self.options.WorkDir = filepath.Dir(path)
		self.options.File = path
		return nil
	}
}

func WithDockerFileContent(content []byte) Option {
	return func(self *Builder) error {
		if content == nil || len(content) == 0 {
			return nil
		}
		temp, err := storage.Local{}.CreateTempDir("")
		if err != nil {
			return err
		}
		self.options.WorkDir = temp
		go func() {
			<-self.ctx.Done()
			err = os.RemoveAll(self.options.WorkDir)
			if err != nil {
				slog.Debug("buildx delete dockerfile temp path", "path", self.options.WorkDir)
			}
		}()
		return os.WriteFile(filepath.Join(self.options.WorkDir, "Dockerfile"), content, 0666)
	}
}

// WithGitUrl https://[username]:[password]@github.com/username/name.git#branchName:path
func WithGitUrl(url string) Option {
	return func(self *Builder) error {
		self.options.WorkDir = url
		return nil
	}
}

func WithZipFilePath(path string) Option {
	return func(self *Builder) error {
		if path == "" {
			return nil
		}
		temp, err := storage.Local{}.CreateTempDir("")
		if err != nil {
			return err
		}
		err = function.Unzip(temp, path)
		if err != nil {
			return err
		}
		defer func() {
			_ = os.Remove(path)
		}()
		self.options.WorkDir = temp
		go func() {
			<-self.ctx.Done()
			err = os.RemoveAll(self.options.WorkDir)
			if err != nil {
				slog.Debug("buildx delete zip temp path", "path", self.options.WorkDir)
			}
		}()
		return nil
	}
}

// WithPlatform 设置目标平台 (支持多个)
func WithPlatform(platforms ...string) Option {
	return func(self *Builder) error {
		self.options.Platforms = append(self.options.Platforms, platforms...)
		return nil
	}
}

func WithBuildArg(args ...types.EnvItem) Option {
	return func(self *Builder) error {
		for _, item := range args {
			// 如果包含 HTTP_PROXY 就透传到环境变量中
			if strings.HasSuffix(strings.ToUpper(item.Name), "_PROXY") {
				self.env = append(self.env, item)
			}
			self.options.BuildArg = append(self.options.BuildArg, item.String())
		}
		return nil
	}
}

// WithBuildSecret 添加 Secret
func WithBuildSecret(args ...types.EnvItem) Option {
	return func(self *Builder) error {
		for _, item := range args {
			// value 需要解一下密
			if v, err := function.RSADecode(item.Value, nil); err == nil {
				item.Value = v
			}
			self.env = append(self.env, item)
			val := fmt.Sprintf("id=%s,env=%s", item.Name, item.Name)
			self.options.Secrets = append(self.options.Secrets, val)
		}
		return nil
	}
}

func WithCache(mode string) Option {
	return func(self *Builder) error {
		self.options.NoCache = false
		self.options.CacheTo = []string{}
		self.options.CacheFrom = []string{}

		var firstTag string
		for _, group := range self.options.Target {
			if len(group.Tags) > 0 {
				firstTag = group.Tags[0]
				break
			}
		}

		switch mode {
		case "none":
			self.options.NoCache = true
		case "default":
		case "inline":
			self.options.CacheTo = append(self.options.CacheTo, "type=inline")
			if firstTag != "" {
				self.options.CacheFrom = append(self.options.CacheFrom, firstTag)
			}
		case "registry":
			if firstTag == "" {
				return errors.New("cache mode 'registry' requires at least one valid image tag")
			}
			if a, _, ok := strings.Cut(firstTag, ":"); ok {
				self.options.CacheTo = append(self.options.CacheTo, fmt.Sprintf("type=registry,ref=%s,mode=max", a+":dpanel-buildcache"))
				self.options.CacheFrom = append(self.options.CacheFrom, fmt.Sprintf("type=registry,ref=%s", a+":dpanel-buildcache"))
			}
		default:
			return nil
		}
		return nil
	}
}

// WithOutputImage 导出为镜像，并可配置是否推送到仓库或压缩
// push: 是否推送到远程仓库 (同 --push)
// compression: 压缩方式 ("gzip", "zstd")
func WithOutputImage(push bool, compression string) Option {
	return func(self *Builder) error {
		if !push {
			return nil
		}
		val := "type=image"
		if push {
			val += ",push=true"
		}
		if compression != "" {
			val += fmt.Sprintf(",compression=%s", compression)
		}
		self.options.Push = true
		self.options.Outputs = append(self.options.Outputs, val)
		return nil
	}
}

func WithRegistryAuth(auth ...registry.AuthConfig) Option {
	return func(self *Builder) error {
		for _, config := range auth {
			if ok := function.InArrayWalk(self.options.RegistryAuth, func(item registry.AuthConfig) bool {
				return item.ServerAddress == config.ServerAddress
			}); !ok {
				self.options.RegistryAuth = append(self.options.RegistryAuth, config)
			}
		}
		return nil
	}
}
