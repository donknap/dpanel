package function

import "net/url"

// LogURL 仅保留请求路径，避免查询参数中的令牌和凭据进入日志。
func LogURL(uri *url.URL) string {
	if uri == nil {
		return ""
	}
	return uri.EscapedPath()
}
