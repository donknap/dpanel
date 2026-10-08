package buildx

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types/registry"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/archive"
	"github.com/donknap/dpanel/common/service/docker/buildx/build"
	"github.com/donknap/dpanel/common/service/docker/types"
	"github.com/donknap/dpanel/common/service/storage"
)

type BuildOptions = build.Options
type BuildOptionsTarget = build.Target

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
		if self.options.WorkDir != "" {
			dockerfileName := path
			if dockerfileName == "" {
				dockerfileName = "Dockerfile"
			}
			dockerfileName = strings.TrimPrefix(dockerfileName, "/")
			if !filepath.IsLocal(dockerfileName) {
				return fmt.Errorf("invalid Dockerfile path %q", path)
			}
			self.options.File = filepath.Join(self.options.WorkDir, dockerfileName)
			return nil
		}
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
		self.options.File = filepath.Join(temp, "Dockerfile")
		self.tempDirs = append(self.tempDirs, temp)
		return os.WriteFile(filepath.Join(temp, "Dockerfile"), content, 0600)
	}
}

// WithGitUrl https://[username]:[password]@github.com/username/name.git#branchName:path
func WithGitUrl(url string) Option {
	return func(self *Builder) error {
		self.options.WorkDir = url
		return nil
	}
}

func WithZipFilePath(path, root string) Option {
	return func(self *Builder) error {
		if path == "" {
			return nil
		}
		temp, err := storage.Local{}.CreateTempDir("")
		if err != nil {
			return err
		}
		self.tempDirs = append(self.tempDirs, temp)
		archivePath := filepath.Join(temp, "context.tar")
		err = archive.ZipToTar(self.ctx, path, archivePath, archive.Option{Root: root})
		if err != nil {
			return err
		}
		contextDir := filepath.Join(temp, "context")
		if err = archive.UnArchiveWithContext(self.ctx, archivePath, contextDir); err != nil {
			return err
		}
		self.options.WorkDir = contextDir
		return os.Remove(archivePath)
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
			value, err := function.RSADecode(item.Value, nil)
			if err != nil {
				value = ""
			}
			item.Value = value
			self.env = append(self.env, item)
			val := fmt.Sprintf("id=%s,env=%s", item.Name, item.Name)
			self.options.Secrets = append(self.options.Secrets, val)
		}
		return nil
	}
}

func WithLabels(labels ...types.EnvItem) Option {
	return func(self *Builder) error {
		for _, label := range labels {
			self.options.Labels = append(self.options.Labels, label.Name+"="+label.Value)
		}
		return nil
	}
}

func WithPull(pull bool) Option {
	return func(self *Builder) error {
		self.options.Pull = pull
		return nil
	}
}

func WithProvenance(enabled *bool) Option {
	return func(self *Builder) error {
		self.options.Provenance = enabled
		return nil
	}
}

func WithExtraArgs(value string) Option {
	return func(self *Builder) error {
		allowed := map[string]bool{
			"--sbom": true, "--annotation": true, "--no-cache-filter": true,
			"--add-host": true, "--network": true, "--shm-size": true,
			"--ulimit": true, "--resource": true,
		}
		for _, line := range strings.Split(value, "\n") {
			arg := strings.TrimSpace(line)
			if arg == "" {
				continue
			}
			flag, parameter, ok := strings.Cut(arg, "=")
			if !ok || !allowed[flag] || parameter == "" || strings.ContainsAny(arg, "\r\x00") {
				return fmt.Errorf("unsupported buildx extension argument %q", arg)
			}
			if flag == "--network" && parameter != "default" && parameter != "none" {
				return fmt.Errorf("unsupported buildx network %q", parameter)
			}
			self.options.ExtraArgs = append(self.options.ExtraArgs, arg)
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

func WithRegistryAuth(auth ...string) Option {
	return func(self *Builder) error {
		for _, authStr := range auth {
			authConfig, err := registry.DecodeAuthConfig(authStr)
			if err != nil {
				return fmt.Errorf("decode registry auth: %w", err)
			}
			if ok := function.InArrayWalk(self.options.RegistryAuth, func(item *registry.AuthConfig) bool {
				return item.ServerAddress == authConfig.ServerAddress
			}); !ok {
				self.options.RegistryAuth = append(self.options.RegistryAuth, authConfig)
			}
		}
		return nil
	}
}
