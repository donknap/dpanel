package accessor

type ImageSettingOption struct {
	BuildGit        string `json:"buildGit"`
	BuildDockerfile string `json:"buildDockerfile"`
	BuildRoot       string `json:"buildRoot"`
	Platform        string `json:"platform"`
	Registry        string `json:"registry"`
}
