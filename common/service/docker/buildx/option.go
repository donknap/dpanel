package buildx

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/docker/types"
	"github.com/donknap/dpanel/common/service/storage"
)

type Option func(self *Builder) error

func WithWorkDir(path string) Option {
	return func(self *Builder) error {
		self.workDir = path
		return nil
	}
}

// WithTag 添加镜像 Tag (支持多个)
func WithTag(tags ...string) Option {
	return func(self *Builder) error {
		self.options.Tags = append(self.options.Tags, tags...)
		return nil
	}
}

// WithDockerFilePath WithDockerFile 指定 Dockerfile 路径
func WithDockerFilePath(path string) Option {
	return func(self *Builder) error {
		self.workDir = filepath.Dir(path)
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
		self.workDir = temp
		go func() {
			<-self.ctx.Done()
			err = os.RemoveAll(self.workDir)
			if err != nil {
				slog.Debug("buildx delete dockerfile temp path", "path", self.workDir)
			}
		}()
		return os.WriteFile(filepath.Join(self.workDir, "Dockerfile"), content, 0666)
	}
}

// WithGitUrl https://[username]:[password]@github.com/username/name.git#branchName:path
func WithGitUrl(url string) Option {
	return func(self *Builder) error {
		self.workDir = url
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
		self.workDir = temp
		go func() {
			<-self.ctx.Done()
			err = os.RemoveAll(self.workDir)
			if err != nil {
				slog.Debug("buildx delete zip temp path", "path", self.workDir)
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

func WithPlatformArm64() Option {
	name := "linux/arm64"
	return func(self *Builder) error {
		if !function.InArray(self.options.Platforms, name) {
			self.options.Platforms = append(self.options.Platforms, name)
		}
		return nil
	}
}

// WithPlatformAmd64 快捷方式：仅构建 linux/amd64
func WithPlatformAmd64() Option {
	name := "linux/amd64"
	return func(self *Builder) error {
		if !function.InArray(self.options.Platforms, name) {
			self.options.Platforms = append(self.options.Platforms, name)
		}
		return nil
	}
}

func WithPlatformArm() Option {
	name := "linux/arm"
	return func(self *Builder) error {
		if !function.InArray(self.options.Platforms, name) {
			self.options.Platforms = append(self.options.Platforms, name)
		}
		return nil
	}
}

func WithBuildArg(args ...types.EnvItem) Option {
	return func(self *Builder) error {
		for _, item := range args {
			self.options.BuildArg = append(self.options.BuildArg, item.String())
		}
		return nil
	}
}

// WithTarget 指定构建阶段
func WithTarget(target string) Option {
	return func(self *Builder) error {
		self.options.Target = target
		return nil
	}
}

// WithSecret 添加 Secret
func WithSecret(args ...types.EnvItem) Option {
	return func(self *Builder) error {
		for _, item := range args {
			val := fmt.Sprintf("id=%s,env=%s", item.Name, item.Name)
			self.options.Secrets = append(self.options.Secrets, val)
		}
		self.env = append(self.env, args...)
		return nil
	}
}

// WithNoCache 禁用缓存
func WithNoCache() Option {
	return func(self *Builder) error {
		self.options.NoCache = true
		return nil
	}
}

// WithCacheRegistry 成对配置 Registry 缓存：从指定镜像拉取缓存，并将新缓存推送到该镜像
func WithCacheRegistry(ref string, mode string) Option {
	return func(self *Builder) error {
		val := fmt.Sprintf("type=registry,ref=%s", ref)
		if mode != "" {
			val += fmt.Sprintf(",mode=%s", mode)
		}
		self.options.CacheFrom = append(self.options.CacheFrom, val)
		self.options.CacheTo = append(self.options.CacheTo, val)
		return nil
	}
}

// WithCacheGHA 成对配置 GitHub Actions 专用缓存 (自动利用 GitHub 缓存 API)
func WithCacheGHA(mode string) Option {
	return func(self *Builder) error {
		val := "type=gha"
		if mode != "" {
			val += fmt.Sprintf(",mode=%s", mode)
		}
		self.options.CacheFrom = append(self.options.CacheFrom, val)
		self.options.CacheTo = append(self.options.CacheTo, val)
		return nil
	}
}

// WithCacheInline 配置内联缓存 (缓存直接嵌入镜像中)
// 注意：inline 模式不需要 cache-from，因为它直接从镜像本身读取
func WithCacheInline() Option {
	return func(self *Builder) error {
		self.options.CacheTo = append(self.options.CacheTo, "type=inline")
		return nil
	}
}

// WithOutputDocker 导出为 Docker 镜像格式
func WithOutputDocker() Option {
	return func(self *Builder) error {
		self.options.Outputs = append(self.options.Outputs, "type=docker")
		return nil
	}
}

// WithOutputOCI 导出为 OCI 兼容的镜像布局文件夹或文件
func WithOutputOCI(dest string) Option {
	return func(self *Builder) error {
		self.options.Outputs = append(self.options.Outputs, fmt.Sprintf("type=oci,dest=%s", dest))
		return nil
	}
}

// WithOutputImage 导出为镜像，并可配置是否推送到仓库或压缩
// push: 是否推送到远程仓库 (同 --push)
// compression: 压缩方式 ("gzip", "zstd")
func WithOutputImage(push bool, compression string) Option {
	return func(self *Builder) error {
		val := "type=image"
		if push {
			val += ",push=true"
		}
		if compression != "" {
			val += fmt.Sprintf(",compression=%s", compression)
		}
		self.options.Outputs = append(self.options.Outputs, val)
		return nil
	}
}
