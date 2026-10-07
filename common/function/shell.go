package function

import "strings"

// ShellQuote 将字符串引用为 POSIX shell 的单个参数。
func ShellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

// PowerShellQuote 将字符串引用为 PowerShell 的单个参数。
func PowerShellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}
