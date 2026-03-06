package function

import (
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/compose-spec/compose-go/v2/paths"
	"github.com/donknap/dpanel/common/library/sanitize"
)

// PathConvertWinPath2Unix 转换 windows 路径 c:\\my\\path\\shiny 为 /c/my/path/shiny
func PathConvertWinPath2Unix(p string) (string, bool) {
	if !paths.IsWindowsAbs(p) {
		return p, false
	}
	pathName, pathValue, ok := strings.Cut(p, ":\\")
	if !ok {
		// 再尝试用 d:/ 来切割
		pathName, pathValue, ok = strings.Cut(p, ":/")
		if !ok {
			return p, false
		}
	}
	convertedSource := fmt.Sprintf("/%s/%s", strings.ToLower(pathName), strings.ReplaceAll(pathValue, "\\", "/"))
	return path.Clean(convertedSource), true
}

// Path2SystemSafe 传入类 linux 风格的路径，返回一个当前系统支持的安全路径
func Path2SystemSafe(p string) string {
	if p == "" {
		return "."
	}
	if len(p) < 2 {
		return filepath.Clean(p)
	}
	p = filepath.ToSlash(p)
	if runtime.GOOS == "windows" && p[0] == '/' {
		// 无论 /d/abc 还是 /d，统一处理
		// 核心逻辑：取第2位作为盘符，拼接冒号，再接剩下的部分
		if len(p) == 2 {
			p = string(p[1]) + ":/"
		} else if p[2] == '/' {
			p = string(p[1]) + ":" + p[2:]
		}
		// Windows 没办法清除路径
		return p
	} else {
		return PathClean(filepath.FromSlash(p))
	}
}

func PathClean(p string) string {
	const underscorePlaceholder = "DPanelUnderscorePlaceholder"
	safeP := strings.ReplaceAll(p, "_", strings.ToLower(underscorePlaceholder))

	var cleaned string
	if runtime.GOOS == "windows" && (filepath.VolumeName(p) != "" || strings.Contains(p, "\\")) {
		vol := filepath.VolumeName(safeP)
		rest := safeP[len(vol):]

		// 处理 Windows 内部逻辑
		cleanedRest := sanitize.Path(filepath.ToSlash(rest))
		cleanedRest = filepath.FromSlash(cleanedRest)

		// 补回被截掉的根路径分隔符
		if len(rest) > 0 && (rest[0] == '/' || rest[0] == '\\') {
			if len(cleanedRest) == 0 || !(cleanedRest[0] == '/' || cleanedRest[0] == '\\') {
				cleaned = vol + string(filepath.Separator) + cleanedRest
			} else {
				cleaned = vol + cleanedRest
			}
		} else {
			cleaned = vol + cleanedRest
		}
	} else {
		// 如果是 Linux 系统，或者是 Windows 下的 Linux 风格路径 (如 /etc/nginx)
		// 直接使用 sanitize 处理，它能完美处理这种斜杠开头的路径
		cleaned = sanitize.Path(safeP)
	}

	// 还原下划线
	cleaned = strings.ReplaceAll(cleaned, strings.ToLower(underscorePlaceholder), "_")
	return cleaned
}

func PathSize(p string) (int64, error) {
	var size int64

	err := filepath.WalkDir(p, func(walkPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			info, err := d.Info()
			if err == nil {
				size += info.Size()
			}
		}
		return nil
	})
	return size, err
}
