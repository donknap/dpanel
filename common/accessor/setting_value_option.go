package accessor

type SettingValueOption struct {
	Username       string `json:"username"`
	Password       string `json:"password"`
	ServerIp       string `json:"serverIp"`
	RequestTimeout int    `json:"requestTimeout"`
}
