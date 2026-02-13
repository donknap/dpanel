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
	return sanitize.Path(p)
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
