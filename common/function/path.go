package function

import (
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/compose-spec/compose-go/v2/paths"
)

// PathSeparatorToSlash 在任意运行系统上统一路径分隔符，例如 C:\app\compose.yaml 转为 C:/app/compose.yaml。
// 不同于 filepath.ToSlash，它在非 Windows 系统上也会转换反斜杠；保留盘符、.、.. 和空字符串，不清理或映射目录。
func PathSeparatorToSlash(pathValue string) string {
	return strings.ReplaceAll(pathValue, "\\", "/")
}

// WindowsPathToSlash 将 Windows 盘符绝对路径转换为 /盘符/...，例如 C:\app\compose.yaml 转为 /c/app/compose.yaml。
// 转换时小写盘符并清理 .、.. 和多余分隔符；不支持的输入原样返回，bool 为 false。
// 结果仅遵循项目的盘符映射约定，不代表目录存在，也不添加 /mnt/host 等运行环境前缀。
func WindowsPathToSlash(p string) (string, bool) {
	if !paths.IsWindowsAbs(p) {
		return p, false
	}
	pathName, pathValue, ok := strings.Cut(PathSeparatorToSlash(p), ":/")
	if !ok {
		return p, false
	}
	convertedSource := fmt.Sprintf("/%s/%s", strings.ToLower(pathName), pathValue)
	return path.Clean(convertedSource), true
}

// SlashPathToSystem converts a slash path to a path usable by the current system.
func SlashPathToSystem(p string) string {
	if p == "" {
		return "."
	}
	p = filepath.ToSlash(p)
	if runtime.GOOS == "windows" && len(p) >= 2 && p[0] == '/' &&
		((p[1] >= 'a' && p[1] <= 'z') || (p[1] >= 'A' && p[1] <= 'Z')) {
		if len(p) == 2 {
			p = string(p[1]) + ":/"
		} else if len(p) > 2 && p[2] == '/' {
			p = string(p[1]) + ":" + p[2:]
		}
	}
	return filepath.Clean(filepath.FromSlash(p))
}

func PathClean(p string) string {
	var sb strings.Builder
	// 仅压缩非法字符替换产生的 '-'，纯合法路径中的原始 '-' 需要原样保留。
	lastWasDash := false
	lastDashFromReplacement := false

	// 1. 白名单过滤：仅允许安全字符通过，非法字符（如空格、&、|、" 等）替换为 '-'
	for _, r := range p {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') ||
			r == '/' || r == '\\' || r == ':' || r == '.' || r == '_' || r == '@' {
			sb.WriteRune(r)
			lastWasDash = false
			lastDashFromReplacement = false
		} else if r == '-' {
			if lastDashFromReplacement {
				lastDashFromReplacement = false
				continue
			}
			sb.WriteRune(r)
			lastWasDash = true
			lastDashFromReplacement = false
		} else {
			// 将不在白名单中的危险/非法字符统一替换为 '-'
			if !lastWasDash {
				sb.WriteRune('-')
				lastWasDash = true
			}
			lastDashFromReplacement = true
		}
	}

	cleaned := sb.String()

	if len(cleaned) > 1 {
		cleaned = strings.TrimRight(cleaned, "-")
	}

	for strings.Contains(cleaned, "..") {
		cleaned = strings.ReplaceAll(cleaned, "..", "")
	}

	if strings.HasPrefix(cleaned, "./") {
		cleaned = strings.TrimPrefix(cleaned, "./")
	}

	if cleaned == "" {
		return "."
	}

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
