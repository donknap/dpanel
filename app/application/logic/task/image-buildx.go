package task

import (
	"fmt"
	"path/filepath"

	"github.com/donknap/dpanel/app/application/logic"
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/service/docker/buildx"
	"github.com/donknap/dpanel/common/types/define"
)

func (self Docker) BuildxOptions(task accessor.ImageSettingOption) ([]buildx.Option, error) {
	// Git 的根目录已包含在 URL 中；ZIP 的根目录在解包时处理。
	options := []buildx.Option{
		buildx.WithBuildArg(task.BuildArgs...),
		buildx.WithBuildSecret(task.BuildSecret...),
		buildx.WithPlatform(task.BuildPlatformType...),
		buildx.WithOutputImage(task.BuildEnablePush, ""),
	}

	hasTag := false
	for _, tag := range task.Tags {
		if !tag.Enable {
			continue
		}
		hasTag = true
		if v := (logic.Image{}).GetRegistryConfig(tag.Registry); v != nil {
			options = append(options, buildx.WithRegistryAuth(v.Auth))
		}
		options = append(options, buildx.WithTag(tag.Target, tag.Uri()))
	}
	if !hasTag {
		return nil, define.ErrorImageTagEmpty
	}

	if task.BuildCacheType != "" {
		options = append(options, buildx.WithCache(task.BuildCacheType))
	}
	if task.BuildGit != "" {
		options = append(options, buildx.WithDockerFilePath(task.BuildDockerfileName), buildx.WithGitUrl(task.BuildGit))
	} else {
		dockerfileName := task.BuildDockerfileName
		if dockerfileName == "" {
			dockerfileName = "Dockerfile"
		}
		if !filepath.IsLocal(dockerfileName) {
			return nil, fmt.Errorf("invalid Dockerfile path %q", dockerfileName)
		}
		switch {
		case task.BuildZip != "":
			options = append(options, buildx.WithDockerFilePath(dockerfileName), buildx.WithZipFilePath(task.BuildZip, task.BuildDockerfileRoot))
		case task.BuildPath != "":
			options = append(options, buildx.WithDockerFilePath(filepath.Join(task.BuildPath, dockerfileName)), buildx.WithWorkDir(task.BuildPath))
		default:
			options = append(options, buildx.WithDockerFileContent([]byte(task.BuildDockerfileContent)))
		}
	}
	return options, nil
}
