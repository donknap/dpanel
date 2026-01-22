package buildx

import (
	"fmt"

	"github.com/donknap/dpanel/common/function"
)

type Option func(self *Builder)

func WithWorkDir(path string) Option {
	return func(self *Builder) {
		self.workDir = path
	}
}

// WithTag 添加镜像 Tag (支持多个)
func WithTag(tags ...string) Option {
	return func(self *Builder) {
		self.options.Tags = append(self.options.Tags, tags...)
	}
}

// WithDockerFile 指定 Dockerfile 路径
func WithDockerFile(path string) Option {
	return func(self *Builder) {
		self.options.File = path
	}
}

// WithPlatform 设置目标平台 (支持多个)
func WithPlatform(platforms ...string) Option {
	return func(self *Builder) {
		self.options.Platforms = append(self.options.Platforms, platforms...)
	}
}

func WithPlatformArm64() Option {
	name := "linux/arm64"
	return func(self *Builder) {
		if !function.InArray(self.options.Platforms, name) {
			self.options.Platforms = append(self.options.Platforms, name)
		}
	}
}

// WithPlatformAmd64 快捷方式：仅构建 linux/amd64
func WithPlatformAmd64() Option {
	name := "linux/amd64"
	return func(self *Builder) {
		if !function.InArray(self.options.Platforms, name) {
			self.options.Platforms = append(self.options.Platforms, name)
		}
	}
}

func WithPlatformArm() Option {
	name := "linux/arm"
	return func(self *Builder) {
		if !function.InArray(self.options.Platforms, name) {
			self.options.Platforms = append(self.options.Platforms, name)
		}
	}
}

func WithBuildArg(key, value string) Option {
	return func(self *Builder) {
		self.options.BuildArg = append(self.options.BuildArg, fmt.Sprintf("%s=%s", key, value))
	}
}

// WithPush 设置 --push (构建后推送到 registry)
func WithPush() Option {
	return func(self *Builder) {
		self.options.Push = true
	}
}

// WithNoCache 禁用缓存
func WithNoCache() Option {
	return func(self *Builder) {
		self.options.NoCache = true
	}
}

// WithTarget 指定构建阶段
func WithTarget(target string) Option {
	return func(self *Builder) {
		self.options.Target = target
	}
}

// WithSecret 添加 Secret (format: "id=mysecret,src=/local/secret")
func WithSecret(id, src string) Option {
	return func(self *Builder) {
		val := fmt.Sprintf("id=%s,src=%s", id, src)
		self.options.Secrets = append(self.options.Secrets, val)
	}
}

// WithCacheRegistry 成对配置 Registry 缓存：从指定镜像拉取缓存，并将新缓存推送到该镜像
func WithCacheRegistry(ref string, mode string) Option {
	return func(self *Builder) {
		val := fmt.Sprintf("type=registry,ref=%s", ref)
		if mode != "" {
			val += fmt.Sprintf(",mode=%s", mode)
		}
		self.options.CacheFrom = append(self.options.CacheFrom, val)
		self.options.CacheTo = append(self.options.CacheTo, val)
	}
}

// WithCacheLocal 成对配置本地目录缓存
func WithCacheLocal(path string, mode string) Option {
	return func(self *Builder) {
		from := fmt.Sprintf("type=local,src=%s", path)
		to := fmt.Sprintf("type=local,dest=%s", path)
		if mode != "" {
			to += fmt.Sprintf(",mode=%s", mode)
		}
		self.options.CacheFrom = append(self.options.CacheFrom, from)
		self.options.CacheTo = append(self.options.CacheTo, to)
	}
}

// WithCacheGHA 成对配置 GitHub Actions 专用缓存 (自动利用 GitHub 缓存 API)
func WithCacheGHA(mode string) Option {
	return func(self *Builder) {
		val := "type=gha"
		if mode != "" {
			val += fmt.Sprintf(",mode=%s", mode)
		}
		self.options.CacheFrom = append(self.options.CacheFrom, val)
		self.options.CacheTo = append(self.options.CacheTo, val)
	}
}

// WithCacheInline 配置内联缓存 (缓存直接嵌入镜像中)
// 注意：inline 模式不需要 cache-from，因为它直接从镜像本身读取
func WithCacheInline() Option {
	return func(self *Builder) {
		self.options.CacheTo = append(self.options.CacheTo, "type=inline")
	}
}

// WithOutputLocal 将构建结果导出为本地文件（即提取 Dockerfile 中的文件到宿主机）
// dest: 宿主机存放文件的路径
func WithOutputLocal(dest string) Option {
	return func(self *Builder) {
		self.options.Outputs = append(self.options.Outputs, fmt.Sprintf("type=local,dest=%s", dest))
	}
}

// WithOutputTar 将构建结果打包成 tar 文件导出到宿主机
// dest: tar 文件的存放路径
func WithOutputTar(dest string) Option {
	return func(self *Builder) {
		self.options.Outputs = append(self.options.Outputs, fmt.Sprintf("type=tar,dest=%s", dest))
	}
}

// WithOutputDocker 导出为 Docker 镜像格式
func WithOutputDocker(platform string) Option {
	return func(self *Builder) {
		self.options.Outputs = append(self.options.Outputs, "type=oci,dest=./my-app.tar,platform="+platform)
	}
}

// WithOutputOCI 导出为 OCI 兼容的镜像布局文件夹或文件
func WithOutputOCI(dest string) Option {
	return func(self *Builder) {
		self.options.Outputs = append(self.options.Outputs, fmt.Sprintf("type=oci,dest=%s", dest))
	}
}

// WithOutputImage 导出为镜像，并可配置是否推送到仓库或压缩
// push: 是否推送到远程仓库 (同 --push)
// compression: 压缩方式 ("gzip", "zstd")
func WithOutputImage(push bool, compression string) Option {
	return func(self *Builder) {
		val := "type=image"
		if push {
			val += ",push=true"
		}
		if compression != "" {
			val += fmt.Sprintf(",compression=%s", compression)
		}
		self.options.Outputs = append(self.options.Outputs, val)
	}
}
