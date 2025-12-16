package function

import (
	"fmt"
	"path"
	"strings"

	"github.com/compose-spec/compose-go/v2/paths"
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
