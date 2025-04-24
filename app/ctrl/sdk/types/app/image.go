package app

type ImageCheckUpgradeOption struct {
	Tag       string `json:"tag"` // 要检测的镜像 tag
	Md5       string `json:"md5"` // 镜像 id
	CacheTime int    `json:"cacheTime"`
}

type ImageCheckUpgradeResult struct {
	Upgrade     bool     `json:"upgrade"`
	Digest      string   `json:"digest"`
	DigestLocal []string `json:"digestLocal"`
}

type ImageTagRemoteOption struct {
	Tag      string `json:"tag"`
	Type     string `json:"type"` // pull or push
	Platform string `json:"platform"`
}

type ImageTagRemoteResult struct {
	Tag      string `json:"tag"`
	ProxyUrl string `json:"proxyUrl"`
}
