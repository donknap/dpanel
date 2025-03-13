package accessor

type RegistrySettingOption struct {
	Username   string   `json:"username"`
	Password   string   `json:"password"`
	Email      string   `json:"email"`
	Proxy      []string `json:"proxy"`
	EnableHttp bool     `json:"enableHttp"`
}
