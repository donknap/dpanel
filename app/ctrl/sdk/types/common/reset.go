package common

type ResetOption struct {
	User       string  `json:"user,omitempty"`
	Password   string  `json:"password,omitempty"`
	Entrance   *string `json:"entrance,omitempty"`
	Cache      bool    `json:"cache,omitempty"`
	OnlineUser bool    `json:"onlineUser,omitempty"`
}
