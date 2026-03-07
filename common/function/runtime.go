package function

import "runtime"

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
