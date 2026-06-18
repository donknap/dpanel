package docker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"strings"

	imagecopy "github.com/containers/image/v5/copy"
	dockerarchive "github.com/containers/image/v5/docker/archive"
	"github.com/containers/image/v5/docker/reference" // [核心]: 用于严格解析和校验镜像名称
	cimgmanifest "github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/oci/archive"
	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/types"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/mholt/archives"
	"github.com/opencontainers/go-digest"

	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

// ErrOciArchiveNotSplittable 表示 OCI 包不能转换，应回退到 docker load。
var ErrOciArchiveNotSplittable = errors.New("oci archive cannot be split")

// OciToDockerTar 将多平台 oci 提取成标准的 docker tar 包
func (self Client) OciToDockerTar(ctx context.Context, tarPath string) (*os.File, error) {
	currentOS, currentArch := function.CurrentSystemPlatform()
	if ok, reason := canSplitOciArchive(ctx, tarPath, currentOS, currentArch); !ok {
		return nil, fmt.Errorf("%w: %s", ErrOciArchiveNotSplittable, reason)
	}

	targetRef, imageName := self.extractOciMetadata(ctx, tarPath, currentOS, currentArch)
	if imageName == "" {
		return nil, errors.New("no image name found in OCI index")
	}

	parsedRef, err := reference.ParseNormalizedNamed(imageName)
	if err != nil {
		return nil, err
	}

	taggedRef, isTagged := parsedRef.(reference.NamedTagged)
	if !isTagged {
		// 如果名字里没有 Tag (如只有 "nginx")，自动补齐 ":latest"
		taggedRef, err = reference.WithTag(parsedRef, "latest")
		if err != nil {
			return nil, err
		}
	}

	srcRef, err := archive.NewReference(tarPath, targetRef)
	if err != nil {
		return nil, err
	}

	tempTarFile, err := storage.Local{}.CreateTempFile("")
	if err != nil {
		return nil, err
	}
	tempTarPath := tempTarFile.Name()
	tempTarFile.Close()

	destRef, err := dockerarchive.NewReference(tempTarPath, taggedRef)
	if err != nil {
		_ = os.Remove(tempTarPath)
		return nil, err
	}

	policyContext, err := signature.NewPolicyContext(&signature.Policy{
		Default: []signature.PolicyRequirement{signature.NewPRInsecureAcceptAnything()},
	})
	if err != nil {
		_ = os.Remove(tempTarPath)
		return nil, err
	}
	defer policyContext.Destroy()

	sysCtx := &types.SystemContext{
		ArchitectureChoice: currentArch,
		OSChoice:           currentOS,
	}

	_, err = imagecopy.Image(ctx, policyContext, destRef, srcRef, &imagecopy.Options{
		SourceCtx:          sysCtx,
		DestinationCtx:     sysCtx,
		ImageListSelection: imagecopy.CopySystemImage,
	})

	if err != nil {
		_ = os.Remove(tempTarPath)
		return nil, err
	}

	finalFile, err := os.Open(tempTarPath)
	if err != nil {
		_ = os.Remove(tempTarPath)
		return nil, err
	}

	return finalFile, nil
}

func (self Client) OciMimeType(ctx context.Context, tarPath string) (string, error) {
	manifest, mimeType, err := self.OciManifest(ctx, tarPath)
	if err != nil {
		return "unknown", err
	}
	slog.Debug("oci mime", "manifest", manifest)
	return mimeType, nil
}

func (self Client) OciManifest(ctx context.Context, tarPath string) ([]byte, string, error) {
	currentOS, currentArch := function.CurrentSystemPlatform()
	targetRef, _ := self.extractOciMetadata(ctx, tarPath, currentOS, currentArch)

	sysCtx := &types.SystemContext{
		ArchitectureChoice: currentArch,
		OSChoice:           currentOS,
	}

	srcRef, err := archive.NewReference(tarPath, targetRef)
	if err != nil {
		return nil, "", err
	}

	imgSrc, err := srcRef.NewImageSource(ctx, sysCtx)
	if err != nil {
		return nil, "", err
	}
	defer imgSrc.Close()

	return imgSrc.GetManifest(ctx, nil)
}

func (self Client) OciInfo(ctx context.Context, tarPath string) (*imgspecv1.Index, error) {
	file, err := os.Open(tarPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	archiveFS, err := archives.FileSystem(ctx, tarPath, file)
	if err != nil {
		return nil, err
	}
	indexBytes, err := fs.ReadFile(archiveFS, "index.json")
	if err != nil {
		return nil, err
	}
	var layoutIndex imgspecv1.Index
	if err := json.Unmarshal(indexBytes, &layoutIndex); err != nil {
		return nil, err
	}

	return &layoutIndex, nil
}

func canSplitOciArchive(ctx context.Context, tarPath string, currentOS string, currentArch string) (bool, string) {
	file, err := os.Open(tarPath)
	if err != nil {
		return false, err.Error()
	}
	defer file.Close()

	archiveFS, err := archives.FileSystem(ctx, tarPath, file)
	if err != nil {
		return false, err.Error()
	}

	indexBytes, err := fs.ReadFile(archiveFS, "index.json")
	if err != nil {
		return false, err.Error()
	}
	var layoutIndex imgspecv1.Index
	if err := json.Unmarshal(indexBytes, &layoutIndex); err != nil {
		return false, err.Error()
	}

	return canSplitOciIndex(archiveFS, &layoutIndex, currentOS, currentArch)
}

func canSplitOciIndex(archiveFS fs.FS, index *imgspecv1.Index, currentOS string, currentArch string) (bool, string) {
	var lastReason string
	for _, desc := range index.Manifests {
		if desc.Platform != nil && (desc.Platform.OS != currentOS || desc.Platform.Architecture != currentArch) {
			continue
		}

		switch desc.MediaType {
		case imgspecv1.MediaTypeImageIndex, cimgmanifest.DockerV2ListMediaType:
			indexBytes, err := readOciBlob(archiveFS, desc.Digest)
			if err != nil {
				return false, err.Error()
			}
			childIndex := imgspecv1.Index{}
			if err := json.Unmarshal(indexBytes, &childIndex); err != nil {
				return false, err.Error()
			}
			ok, reason := canSplitOciIndex(archiveFS, &childIndex, currentOS, currentArch)
			if ok {
				return true, ""
			}
			lastReason = reason
		case imgspecv1.MediaTypeImageManifest:
			if desc.Platform == nil {
				lastReason = "single OCI manifest has no platform to split"
				continue
			}
			manifestBytes, err := readOciBlob(archiveFS, desc.Digest)
			if err != nil {
				return false, err.Error()
			}
			manifest := imgspecv1.Manifest{}
			if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
				return false, err.Error()
			}
			if ok, reason := isStandardOciManifest(&manifest); !ok {
				return false, reason
			}
			return true, ""
		default:
			lastReason = fmt.Sprintf("unsupported OCI descriptor media type %q", desc.MediaType)
		}
	}
	if lastReason == "" {
		lastReason = "current platform OCI manifest not found"
	}
	return false, lastReason
}

func isStandardOciManifest(manifest *imgspecv1.Manifest) (bool, string) {
	if manifest.MediaType != imgspecv1.MediaTypeImageManifest {
		return false, fmt.Sprintf("unsupported OCI manifest media type %q", manifest.MediaType)
	}

	for _, layer := range manifest.Layers {
		switch layer.MediaType {
		case imgspecv1.MediaTypeImageLayer,
			imgspecv1.MediaTypeImageLayerGzip,
			imgspecv1.MediaTypeImageLayerNonDistributable,
			imgspecv1.MediaTypeImageLayerNonDistributableGzip:
		default:
			return false, fmt.Sprintf("unsupported OCI layer media type %q", layer.MediaType)
		}
	}
	return true, ""
}

func readOciBlob(archiveFS fs.FS, blobDigest digest.Digest) ([]byte, error) {
	return fs.ReadFile(archiveFS, "blobs/sha256/"+blobDigest.Encoded())
}

// extractOciMetadata 辅助提取逻辑 (保持不变)
func (self Client) extractOciMetadata(ctx context.Context, tarPath string, currentOS string, currentArch string) (string, string) {
	var targetRef string
	var imageName string

	v, err := self.OciInfo(ctx, tarPath)
	if err != nil {
		return "", ""
	}

	for _, m := range v.Manifests {
		if m.Annotations != nil && imageName == "" {
			if name := m.Annotations[define.DockerAnnotationRefName]; name != "" {
				if names := strings.Split(name, ","); len(names) > 0 {
					imageName = names[0]
				}
			}
			if refName := m.Annotations[imgspecv1.AnnotationRefName]; refName != "" && imageName == "" {
				if strings.Contains(refName, "/") || strings.Contains(refName, ":") {
					imageName = refName
				}
			}
		}

		if m.Platform != nil && m.Platform.OS == currentOS && m.Platform.Architecture == currentArch {
			targetRef = m.Digest.String()
		}
	}

	if targetRef == "" && len(v.Manifests) > 0 {
		targetRef = v.Manifests[0].Digest.String()
		for _, m := range v.Manifests {
			if ref := m.Annotations[imgspecv1.AnnotationRefName]; ref != "" {
				targetRef = ref
				break
			}
		}
	}

	return targetRef, imageName
}
