package function

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// SafePath 对路径做安全净化。
func SafePath(path string) string {
	return PathClean(path)
}

// SafePathJoin 以 root 为锚点进行安全拼接。
func SafePathJoin(root string, parts ...string) string {
	if root == "" {
		return ""
	}
	cleanRoot := filepath.Clean(root)
	targetPath := filepath.Join(append([]string{cleanRoot}, parts...)...)
	rootAbs, err := filepath.Abs(cleanRoot)
	if err != nil {
		return cleanRoot
	}
	targetAbs, err := filepath.Abs(targetPath)
	if err != nil {
		return cleanRoot
	}
	rootSlash := filepath.ToSlash(rootAbs)
	targetSlash := filepath.ToSlash(targetAbs)
	if targetSlash == rootSlash || strings.HasPrefix(targetSlash, rootSlash+"/") {
		return targetPath
	}
	targetClean := filepath.Clean(targetPath)
	targetVolume := filepath.VolumeName(targetClean)
	targetSlash = filepath.ToSlash(strings.TrimPrefix(targetClean, targetVolume))
	targetSlash = strings.TrimLeft(targetSlash, "/")
	return filepath.Join(cleanRoot, filepath.FromSlash(targetSlash))
}

// SafeFileName 返回净化后的文件名（不包含目录）。
func SafeFileName(name string) string {
	clean := SafePath(name)
	base := filepath.Base(clean)
	if base == "" || base == "." || base == string(filepath.Separator) {
		return "file"
	}
	return base
}

// SafeShell 将参数转换为可安全放入 shell 命令参数位置的值。
// 安全优先：仅兼容明确允许的基础类型，不支持的类型返回空字符串并在模板中跳过。
func SafeShell(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return "'" + strings.ReplaceAll(v, "'", `'"'"'`) + "'"
	case bool:
		if v {
			return "true"
		}
		return "false"
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return ""
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	case json.Number:
		return v.String()
	default:
		return ""
	}
}

// SafeDelete 在 root 根目录内删除 target。
func SafeDelete(root, target string) error {
	rootAbs, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return os.ErrPermission
	}
	if filepath.IsAbs(target) {
		targetAbs, err := filepath.Abs(filepath.Clean(target))
		if err != nil {
			return os.ErrPermission
		}
		targetSlash := filepath.ToSlash(targetAbs)
		rootSlash := filepath.ToSlash(rootAbs)
		if targetSlash == rootSlash {
			return os.ErrPermission
		}
		if strings.HasPrefix(targetSlash, rootSlash+"/") {
			if relPath, err := filepath.Rel(rootAbs, targetAbs); err == nil {
				target = relPath
			}
		}
	}
	safePath := SafePathJoin(root, target)
	targetAbs, err := filepath.Abs(filepath.Clean(safePath))
	if err != nil {
		return os.ErrPermission
	}
	if targetAbs == rootAbs || !strings.HasPrefix(filepath.ToSlash(targetAbs), filepath.ToSlash(rootAbs)+"/") {
		return os.ErrPermission
	}
	return os.Remove(targetAbs)
}

// SafeDeleteAll 在 root 根目录内删除 target 目录树。
func SafeDeleteAll(root, target string) error {
	rootAbs, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return os.ErrPermission
	}
	if filepath.IsAbs(target) {
		targetAbs, err := filepath.Abs(filepath.Clean(target))
		if err != nil {
			return os.ErrPermission
		}
		targetSlash := filepath.ToSlash(targetAbs)
		rootSlash := filepath.ToSlash(rootAbs)
		if targetSlash == rootSlash {
			return os.ErrPermission
		}
		if strings.HasPrefix(targetSlash, rootSlash+"/") {
			if relPath, err := filepath.Rel(rootAbs, targetAbs); err == nil {
				target = relPath
			}
		}
	}
	safePath := SafePathJoin(root, target)
	targetAbs, err := filepath.Abs(filepath.Clean(safePath))
	if err != nil {
		return os.ErrPermission
	}
	if targetAbs == rootAbs || !strings.HasPrefix(filepath.ToSlash(targetAbs), filepath.ToSlash(rootAbs)+"/") {
		return os.ErrPermission
	}
	return os.RemoveAll(targetAbs)
}
