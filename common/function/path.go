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

// WindowsPathToSlash converts a Windows drive path such as c:\my\path to /c/my/path.
func WindowsPathToSlash(p string) (string, bool) {
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
