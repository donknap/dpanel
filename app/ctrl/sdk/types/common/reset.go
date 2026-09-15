package common

type ResetOption struct {
	Entrance   *string `json:"entrance,omitempty"`
	Cache      bool    `json:"cache,omitempty"`
	OnlineUser bool    `json:"onlineUser,omitempty"`
}
