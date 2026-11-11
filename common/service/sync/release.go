package sync

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"time"

	"github.com/donknap/dpanel/common/service/exec/local"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

func Release(ctx context.Context, source, target string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if IsGit(source) {
		cmd, err := local.New(
			local.WithCommandName("git"),
			local.WithArgs("-C", target, "rev-parse", "HEAD"),
			local.WithEnv(os.Environ()),
			local.WithCtx(ctx),
		)
		if err != nil {
			return "", err
		}
		defer cmd.Close()
		content, err := cmd.RunWithResult()
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(content)), nil
	}
	if IsOci(source) {
		platform := v1.Platform{OS: "linux", Architecture: "amd64"}
		content, err := crane.Manifest(source, crane.WithContext(ctx), crane.WithPlatform(&platform))
		if err != nil {
			return "", err
		}
		var manifest v1.Manifest
		if err = json.Unmarshal(content, &manifest); err != nil {
			return "", err
		}
		return manifest.Annotations[define.PanelLabelRelease], nil
	}
	return "", nil
}
