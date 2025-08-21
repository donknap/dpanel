package function

import "os"

func FileExists(file ...string) bool {
	var err error
	for _, value := range file {
		if _, err = os.Stat(value); err != nil {
			return false
		}
	}
	return true
}
