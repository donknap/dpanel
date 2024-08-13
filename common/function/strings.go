package function

import (
	"strings"
)

func CommandSplit(cmd string) []string {
	result := make([]string, 0)

	field := ""
	ignoreSpace := false

	for _, s := range strings.Split(cmd, "") {
		if s == " " && !ignoreSpace {
			result = append(result, field)
			field = ""
			continue
		}

		if s == "\"" || s == "'" {
			ignoreSpace = !ignoreSpace
			continue
		}

		field += s
	}

	if field != "" {
		result = append(result, field)
	}

	return result
}
