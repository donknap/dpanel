package function

import "strings"

func StringReplaceAll(s, old, new string) string {
	if !strings.Contains(s, old) {
		return s
	}
	return strings.ReplaceAll(s, old, new)
}
