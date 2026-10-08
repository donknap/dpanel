package task

import (
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
		buildx.WithLabels(task.BuildLabels...),
		buildx.WithPull(task.BuildPull),
		buildx.WithProvenance(task.BuildProvenance),
		buildx.WithExtraArgs(task.BuildExtraArgs),
		buildx.WithPlatform(task.BuildPlatformType...),
		buildx.WithOutputImage(task.BuildEnablePush, ""),
	}

	hasTag := false
	for _, tag := range task.Tags {
		if !tag.Enable {
			continue
		}
		hasTag = true
		if v := (logic.Image{}).GetRegistryConfig(tag.Registry); v != nil && v.Auth != "" {
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
		switch {
		case task.BuildZip != "":
			options = append(options, buildx.WithZipFilePath(task.BuildZip, task.BuildDockerfileRoot), buildx.WithDockerFilePath(task.BuildDockerfileName))
		case task.BuildPath != "":
			options = append(options, buildx.WithWorkDir(task.BuildPath), buildx.WithDockerFilePath(task.BuildDockerfileName))
		default:
			options = append(options, buildx.WithDockerFileContent([]byte(task.BuildDockerfileContent)))
		}
	}
	return options, nil
}
