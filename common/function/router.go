package function

import (
	"path"

	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
)

func RouterBaseurl() string {
	base := "/"
	if v := facade.Config.GetString("system.baseurl"); v != "" {
		base = path.Join("/", v)
	}
	return path.Join(base)
}

func RouterUri(url string) string {
	return path.Join(RouterBaseurl(), url)
}

func RouterRootApi() string {
	return path.Join(RouterBaseurl(), "api")
}

func RouterRootWs() string {
	return path.Join(RouterBaseurl(), "ws")
}

func RouterApiUri(url string) string {
	return path.Join(RouterRootApi(), url)
}
