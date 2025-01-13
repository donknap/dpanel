package function

import (
	"strings"
)

func ErrorHasKeyword(e error, keyword ...string) bool {
	for _, k := range keyword {
		if strings.Contains(e.Error(), k) {
			return true
		}
	}
	return false
}
