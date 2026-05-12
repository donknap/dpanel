package function

import (
	"encoding/json"
	"os"
	"path/filepath"
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
	all := make([]string, 0, len(parts)+1)
	all = append(all, cleanRoot)
	for _, p := range parts {
		if p == "" {
			continue
		}
		cleanPart := SystemPathFromSlash(p)
		cleanPart = strings.TrimLeft(filepath.ToSlash(cleanPart), "/")
		cleanPart = strings.TrimPrefix(cleanPart, cleanRoot)
		cleanPart = strings.TrimLeft(cleanPart, "/")
		cleanPart = strings.TrimPrefix(cleanPart, filepath.ToSlash(cleanRoot))
		cleanPart = strings.TrimLeft(cleanPart, "/")
		if cleanPart == "" || cleanPart == "." {
			continue
		}
		all = append(all, filepath.FromSlash(cleanPart))
	}
	return filepath.Join(all...)
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
	case json.Number:
		return v.String()
	default:
		return ""
	}
}

// SafeDelete 在 root 根目录内删除 target。
func SafeDelete(root, target string) error {
	safePath := SafePathJoin(root, target)
	rootAbs, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return os.ErrPermission
	}
	targetAbs, err := filepath.Abs(filepath.Clean(safePath))
	if err != nil {
		return os.ErrPermission
	}
	if targetAbs != rootAbs && !strings.HasPrefix(targetAbs, rootAbs+string(filepath.Separator)) {
		return os.ErrPermission
	}
	return os.Remove(targetAbs)
}

// SafeDeleteAll 在 root 根目录内删除 target 目录树。
func SafeDeleteAll(root, target string) error {
	safePath := SafePathJoin(root, target)
	rootAbs, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return os.ErrPermission
	}
	targetAbs, err := filepath.Abs(filepath.Clean(safePath))
	if err != nil {
		return os.ErrPermission
	}
	if targetAbs != rootAbs && !strings.HasPrefix(targetAbs, rootAbs+string(filepath.Separator)) {
		return os.ErrPermission
	}
	return os.RemoveAll(targetAbs)
}
