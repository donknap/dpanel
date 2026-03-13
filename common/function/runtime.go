package function

import (
	"context"
	"runtime"
	"time"
)

func CurrentSystemPlatform() (string, string) {
	currentOS := runtime.GOOS
	currentArch := runtime.GOARCH

	if currentOS == "windows" || currentOS == "darwin" {
		currentOS = "linux"
	}

	switch currentArch {
	case "amd64":
		currentArch = "amd64"
	case "arm64":
		currentArch = "arm64"
	case "386":
		currentArch = "386"
	case "arm":
		currentArch = "arm"
	default:
		// 默认保持原样
	}

	return currentOS, currentArch
}

func Wait[T any](ctx context.Context, data T, conditions func(v T) bool) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if done := conditions(data); done {
				return
			}
		}
	}
}
