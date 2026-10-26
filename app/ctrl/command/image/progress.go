package image

import (
	"fmt"
	"io"

	"github.com/donknap/dpanel/common/service/docker/types"
)

func PrintPullProgress(out io.Writer, imageName string, progressMap map[string]*types.PullProgress) {
	if len(progressMap) == 0 {
		return
	}
	downloadCount := 0
	extractCount := 0
	downloadPercent := 0.0
	extractPercent := 0.0
	for _, progress := range progressMap {
		if progress == nil {
			continue
		}
		downloadPercent += progress.Downloading
		extractPercent += progress.Extracting
		if progress.Downloading >= 100 {
			downloadCount++
		}
		if progress.Extracting >= 100 {
			extractCount++
		}
	}
	_, _ = fmt.Fprintf(out, "Pulling %s: average layer download %.0f%% (%d/%d complete), extract %.0f%% (%d/%d complete)\n",
		imageName, downloadPercent/float64(len(progressMap)), downloadCount, len(progressMap),
		extractPercent/float64(len(progressMap)), extractCount, len(progressMap))
}
