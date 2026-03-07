package docker

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"strings"

	imagecopy "github.com/containers/image/v5/copy"
	"github.com/containers/image/v5/docker/daemon"
	"github.com/containers/image/v5/oci/archive"
	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/types"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/mholt/archives"

	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

func (self Client) OciLoadImage(ctx context.Context, tarPath string) error {
	var imageName string

	if v, err := self.OciInfo(ctx, tarPath); err == nil {
		for _, m := range v.Manifests {
			if m.Annotations == nil {
				continue
			}

			if name := m.Annotations[define.DockerAnnotationRefName]; name != "" {
				names := strings.Split(name, ",")
				if len(names) > 0 {
					imageName = names[0]
					break
				}
			}

			if refName := m.Annotations[imgspecv1.AnnotationRefName]; refName != "" {
				if strings.Contains(refName, "/") || strings.Contains(refName, ":") {
					imageName = refName
				}
			}
		}
	}

	if imageName == "" {
		return errors.New("no image name found")
	}

	srcRef, err := archive.NewReference(tarPath, "")
	if err != nil {
		return err
	}

	destRef, err := daemon.ParseReference(imageName)
	if err != nil {
		return err
	}

	policyContext, err := signature.NewPolicyContext(&signature.Policy{
		Default: []signature.PolicyRequirement{signature.NewPRInsecureAcceptAnything()},
	})
	if err != nil {
		return err
	}
	defer policyContext.Destroy()

	currentOS, currentArch := function.CurrentSystemPlatform()
	sysCtx := &types.SystemContext{
		ArchitectureChoice: currentArch,
		OSChoice:           currentOS,
		DockerDaemonHost:   self.Client.DaemonHost(),
	}

	_, err = imagecopy.Image(ctx, policyContext, destRef, srcRef, &imagecopy.Options{
		SourceCtx:      sysCtx,
		DestinationCtx: sysCtx,
	})

	return err
}

func (self Client) OciMimeType(ctx context.Context, tarPath string) (string, error) {
	_, mimeType, err := self.OciManifest(ctx, tarPath)
	if err != nil {
		return "unknow", err
	}
	return mimeType, nil
}

func (self Client) OciManifest(ctx context.Context, tarPath string) ([]byte, string, error) {
	sysCtx := &types.SystemContext{}

	srcRef, err := archive.NewReference(tarPath, "")
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
